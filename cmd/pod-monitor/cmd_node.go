package main

import (
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"github.com/spf13/cobra"

	"pod-monitor/internal/log"
	"pod-monitor/pkg/k8s"
	"pod-monitor/pkg/output"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "监控节点资源使用情况",
	Long:  `显示 Kubernetes 节点的资源使用情况，包括 CPU、内存使用率和节点状态`,
	Run:   runNodeMonitor,
}

func init() {
	rootCmd.AddCommand(nodeCmd)
}

// NodeMetrics 节点资源使用情况
type NodeMetrics struct {
	NodeName    string
	IP          string
	CPU         int64
	Memory      int64
	TotalCPU    int64
	TotalMemory int64
	MemoryUsage float64
	CPUUsage    float64
	Status      string
}

func runNodeMonitor(cmd *cobra.Command, args []string) {
	if verbose {
		log.Stdout.Info("创建 Kubernetes 客户端")
	}

	client, err := k8s.NewClient(kubeconfig, 50, 100)
	if err != nil {
		log.Stdout.Error("创建 K8S 客户端失败", "error", err)
		os.Exit(1)
	}

	nodes, err := client.Clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Stdout.Error("获取节点列表失败", "error", err)
		os.Exit(1)
	}

	if verbose {
		log.Stdout.Info("开始收集节点指标", "total", len(nodes.Items))
	}

	// 收集所有节点指标
	var nodeMetricsList []*NodeMetrics
	for _, node := range nodes.Items {
		metrics, err := getNodeMetrics(node.Name, client)
		if err != nil {
			if verbose {
				log.Stdout.Warn("获取节点资源指标失败", "node", node.Name, "error", err)
			}
			// 即使metrics获取失败，也显示基本信息
			metrics = &NodeMetrics{
				NodeName: node.Name,
				IP:       getNodeIP(client.Clientset, node.Name),
				Status:   getNodeStatus(&node),
			}
		}
		nodeMetricsList = append(nodeMetricsList, metrics)
	}

	log.Stdout.Info("节点监控完成", "total", len(nodeMetricsList))

	displayNodeResults(nodeMetricsList)
}

// getNodeMetrics 获取节点资源使用情况
func getNodeMetrics(nodeName string, client *k8s.Client) (*NodeMetrics, error) {
	// 获取metrics
	metrics, err := client.Metrics.MetricsV1beta1().NodeMetricses().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 获取节点详情
	node, err := client.Clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// CPU使用量（毫核）
	cpuUsage := metrics.Usage.Cpu().MilliValue()

	// 内存使用量（Mi）
	memoryUsage := metrics.Usage.Memory().Value() / (1024 * 1024)

	// 节点总容量
	totalCPU := node.Status.Capacity.Cpu().MilliValue()
	totalMemory := node.Status.Allocatable.Memory().Value() / (1024 * 1024)

	// 计算使用率
	cpuUsagePercent := float64(cpuUsage) / float64(totalCPU) * 100
	memoryUsagePercent := float64(memoryUsage) / float64(totalMemory) * 100

	return &NodeMetrics{
		NodeName:    nodeName,
		IP:          getNodeIP(client.Clientset, nodeName),
		CPU:         cpuUsage,
		Memory:      memoryUsage,
		TotalCPU:    totalCPU,
		TotalMemory: totalMemory,
		CPUUsage:    cpuUsagePercent,
		MemoryUsage: memoryUsagePercent,
		Status:      getNodeStatus(node),
	}, nil
}

// getNodeIP 获取节点IP地址
func getNodeIP(clientset *kubernetes.Clientset, nodeName string) string {
	if nodeName == "" {
		return "N/A"
	}

	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "Unknown"
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
	}
	return "N/A"
}

// getNodeStatus 获取节点状态
func getNodeStatus(node *v1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if condition.Status == v1.ConditionTrue {
				return "正常"
			}
			return "异常"
		}
	}
	return "Unknown"
}

// displayNodeResults 显示节点监控结果
func displayNodeResults(results []*NodeMetrics) {
	table := output.NewTableWriter(os.Stdout)
	table.SetNodeColumns()
	output.ApplyNodeColors(table)

	for _, r := range results {
		table.Append([]string{
			r.NodeName,
			r.IP,
			fmt.Sprintf("%dm", r.CPU),
			fmt.Sprintf("%dm", r.TotalCPU),
			fmt.Sprintf("%.0f%%", r.CPUUsage),
			fmt.Sprintf("%dMi", r.Memory),
			fmt.Sprintf("%dMi", r.TotalMemory),
			fmt.Sprintf("%.0f%%", r.MemoryUsage),
			r.Status,
		})
	}

	table.Render()
}
