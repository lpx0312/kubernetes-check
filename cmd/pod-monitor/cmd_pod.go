package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"github.com/spf13/cobra"

	"pod-monitor/internal/log"
	"pod-monitor/pkg/k8s"
	"pod-monitor/pkg/output"
)

var (
	podDays       int
	podAbnormal   bool
	allNamespaces bool
	namespace     string
)

var podCmd = &cobra.Command{
	Use:   "pod",
	Short: "检查 Pod 状态",
	Long:  `检查 Kubernetes Pod 的重启情况和异常状态`,
	Run:   runPodCheck,
}

func init() {
	rootCmd.AddCommand(podCmd)

	podCmd.Flags().IntVarP(&podDays, "days", "d", 7, "显示最近 N 天内重启的 Pod")
	podCmd.Flags().BoolVarP(&podAbnormal, "abnormal", "a", false, "仅显示异常状态的 Pod")
	podCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "查看所有命名空间的 Pod")
	podCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "指定要查看的命名空间")
}

type PodResult struct {
	Namespace      string
	Name           string
	Phase          v1.PodPhase
	NodeIP         string
	RestartCount   int
	RestartTime    time.Time
	RestartReason  string
	ReadyStatus    string
	Age            time.Duration
	ContainerStatus string
	IsAbnormal     bool
}

func runPodCheck(cmd *cobra.Command, args []string) {
	if verbose {
		log.Stdout.Info("创建 Kubernetes 客户端")
	}

	client, err := k8s.NewClient(kubeconfig, 50, 100)
	if err != nil {
		log.Stdout.Error("创建 K8S 客户端失败", "error", err)
		os.Exit(1)
	}

	ns := namespace
	if allNamespaces {
		ns = ""
		if verbose {
			log.Stdout.Info("查看所有命名空间的 Pod")
		}
	} else {
		if verbose {
			log.Stdout.Info("查看命名空间", "namespace", ns)
		}
	}

	pods, err := client.Clientset.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		log.Stdout.Error("获取 Pod 列表失败", "error", err)
		os.Exit(1)
	}

	log.Stdout.Info("开始分析 Pod", "total", len(pods.Items))

	// 预加载节点缓存
	nodeCache := sync.Map{}
	preloadNodeCache(client.Clientset, &nodeCache)

	// 创建结果通道和 pod channel
	resultChan := make(chan *PodResult, len(pods.Items))
	podsChan := make(chan v1.Pod, len(pods.Items))

	// 启动 worker pool
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pod := range podsChan {
				result := analyzePod(pod, &nodeCache)
				if result != nil {
					resultChan <- result
				}
			}
		}()
	}

	// 分发 pods 到 channel
	go func() {
		for _, pod := range pods.Items {
			podsChan <- pod
		}
		close(podsChan)
	}()

	// 等待所有 worker 完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var results []*PodResult
	for result := range resultChan {
		results = append(results, result)
	}

	log.Stdout.Info("分析完成", "total", len(pods.Items), "filtered", len(results))

	displayPodResults(results)
}

func analyzePod(pod v1.Pod, nodeCache *sync.Map) *PodResult {
	result := &PodResult{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Phase:     pod.Status.Phase,
		Age:       time.Since(pod.CreationTimestamp.Time),
	}

	// 获取节点IP
	if cached, ok := nodeCache.Load(pod.Spec.NodeName); ok {
		result.NodeIP = cached.(string)
	} else {
		result.NodeIP = "Unknown"
	}

	// 检查是否异常
	result.IsAbnormal = isPodAbnormal(pod)
	result.ReadyStatus = getReadyStatus(pod)

	if podAbnormal {
		if result.IsAbnormal {
			result.ContainerStatus = getContainerStatus(pod)
			return result
		}
		return nil
	}

	// 检查重启
	restartCount, restartTime, restartReason := analyzeContainers(pod.Status.ContainerStatuses, podDays)
	if restartCount > 0 {
		result.RestartCount = restartCount
		result.RestartTime = restartTime
		result.RestartReason = restartReason
		return result
	}

	return nil
}

func isPodAbnormal(pod v1.Pod) bool {
	// 标记正在被删除的 Pod 为异常
	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded {
		return true
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return true
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return true
		}
	}

	ready := getReadyStatus(pod)
	expectedReady := fmt.Sprintf("%d/%d", len(pod.Spec.Containers), len(pod.Spec.Containers))
	if ready != expectedReady {
		return true
	}

	return false
}

func getReadyStatus(pod v1.Pod) string {
	readyContainers := 0
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			readyContainers = len(pod.Spec.Containers)
			break
		}
	}
	return fmt.Sprintf("%d/%d", readyContainers, len(pod.Spec.Containers))
}

func getContainerStatus(pod v1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil {
			return cs.State.Terminated.Reason
		}
	}
	return "N/A"
}

func analyzeContainers(statuses []v1.ContainerStatus, days int) (int, time.Time, string) {
	if len(statuses) == 0 {
		return 0, time.Time{}, ""
	}

	totalRestarts := 0
	var latestRestartTime time.Time
	restartReason := ""

	for _, cs := range statuses {
		totalRestarts += int(cs.RestartCount)

		if cs.LastTerminationState.Terminated != nil {
			terminated := cs.LastTerminationState.Terminated
			if !terminated.FinishedAt.IsZero() {
				finishedAt := terminated.FinishedAt.Time.UTC()
				if finishedAt.After(latestRestartTime) {
					latestRestartTime = finishedAt
					restartReason = terminated.Reason
				}
			}
		}
	}

	cutoffTime := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	if totalRestarts > 0 && !latestRestartTime.IsZero() && latestRestartTime.After(cutoffTime) {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		latestRestartTime = latestRestartTime.In(loc)
		return totalRestarts, latestRestartTime, restartReason
	}
	return 0, time.Time{}, ""
}

func preloadNodeCache(clientset *kubernetes.Clientset, cache *sync.Map) {
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		if verbose {
			log.Stdout.Warn("预加载节点信息失败", "error", err)
		}
		return
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				cache.Store(node.Name, addr.Address)
				break
			}
		}
	}

	if verbose {
		log.Stdout.Info("预加载节点信息完成", "count", len(nodes.Items))
	}
}

func displayPodResults(results []*PodResult) {
	table := output.NewTableWriter(os.Stdout)
	table.SetPodColumns(podAbnormal)
	output.ApplyPodColors(table, podAbnormal)

	for _, r := range results {
		if podAbnormal {
			table.Append([]string{
				r.Namespace,
				r.Name,
				string(r.Phase),
				r.NodeIP,
				r.ReadyStatus,
				formatDuration(r.Age),
				r.ContainerStatus,
			})
		} else {
			table.Append([]string{
				r.Namespace,
				r.Name,
				string(r.Phase),
				r.NodeIP,
				fmt.Sprintf("%d", r.RestartCount),
				formatTime(r.RestartTime),
				r.RestartReason,
				r.ReadyStatus,
			})
		}
	}

	table.Render()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	hours := int(d.Hours())
	return fmt.Sprintf("%dd%dh", hours/24, hours%24)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return t.In(loc).Format("2006-01-02 15:04:05") + " (UTC+8)"
}

