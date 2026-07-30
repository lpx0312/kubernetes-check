// Package collector 负责从 Kubernetes 集群采集巡检数据，产出 report.Report。
// 它是三层架构（Collector → Report → Renderer）的数据生产端：
//   - 封装 clientset / metrics client 的构造
//   - 保留 worker pool 并发模型处理 Pod
//   - 消除历史全局状态泄漏（daysFlag/abnormalFlag 指针收进 Config）
package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"k8s-patrol/internal/report"
)

// 常量（从旧 main.go 平移，供 pod.go/node.go 共用）
const (
	timeFormat = "2006-01-02 15:04:05"
	beijingTZ  = "Asia/Shanghai"
)

// Config 巡检采集配置，由 main 在 flag 解析后构造一次，消除全局 flag 指针泄漏。
type Config struct {
	Kubeconfig   string // kubeconfig 绝对路径，空则用默认路径
	Namespace    string // 目标命名空间（AllNamespace=true 时忽略）
	AllNamespace bool   // 是否查看所有命名空间
	Days         int    // 重启检查的回溯天数
	Workers      int    // worker pool 并发度
	Mode         string // report.Mode* 之一
}

// Collector 巡检采集器，持有 K8s 客户端与节点缓存。
// nodeCache 在 worker pool 间共享，sync.Map 保证并发安全。
type Collector struct {
	clientset *kubernetes.Clientset
	metrics   *metrics.Clientset
	nodeCache sync.Map
	cfg       Config
	cluster   string // kubeconfig 当前 context 名，报告头展示用
}

// New 构造 Collector：加载 kubeconfig、创建 clientset 与 metrics client、提升速率限制。
func New(cfg Config) (*Collector, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.Kubeconfig != "" {
		loadingRules.ExplicitPath = cfg.Kubeconfig
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	// 提取 context 名用于报告头
	cluster := ""
	if rawCfg, err := cc.RawConfig(); err == nil && rawCfg.CurrentContext != "" {
		cluster = rawCfg.CurrentContext
	}

	config, err := cc.ClientConfig()
	if err != nil {
		return nil, err
	}

	// 提升 API 请求速率限制（默认 QPS=5/Burst=10，处理大规模集群不够用）
	config.QPS = 50
	config.Burst = 100

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Collector{
		clientset: clientset,
		metrics:   metricsClient,
		cfg:       cfg,
		cluster:   cluster,
	}, nil
}

// Collect 执行巡检采集，返回完整 Report。根据 cfg.Mode 分派到不同采集路径。
func (c *Collector) Collect(ctx context.Context) (*report.Report, error) {
	rep := &report.Report{
		Mode:        c.cfg.Mode,
		Cluster:     c.cluster,
		GeneratedAt: nowInBeijing(),
	}

	switch c.cfg.Mode {
	case report.ModeNode:
		c.collectNodes(ctx, rep)
	case report.ModeAbnormal:
		c.collectPods(ctx, rep, true)
	case report.ModeRestart:
		c.collectPods(ctx, rep, false)
	case report.ModeFull:
		// 全量巡检：一次采齐节点 + 异常 + 重启，复用现有采集方法
		c.collectNodes(ctx, rep)
		c.collectPods(ctx, rep, true)  // 异常
		c.collectPods(ctx, rep, false) // 重启
		c.computeSummary(rep)
	default:
		c.collectPods(ctx, rep, false)
	}

	return rep, nil
}

// computeSummary 计算全量模式的巡检摘要。
// 健康度判定：存在异常节点(NotReady)或 metrics-server 缺失为"严重"；
// 存在异常 Pod 或重启 Pod 为"警告"；否则为"健康"。
func (c *Collector) computeSummary(rep *report.Report) {
	abnormalNodes := 0
	for _, n := range rep.NodeRows {
		if n.Status != "正常" {
			abnormalNodes++
		}
	}

	summary := report.ReportSummary{
		TotalNodes:    len(rep.NodeRows),
		AbnormalNodes: abnormalNodes,
		TotalPods:     rep.TotalPods,
		AbnormalPods:  len(rep.AbnormalRows),
		RestartedPods: len(rep.RestartRows),
	}

	// 健康度判定（优先级：严重 > 警告 > 健康）
	switch {
	case abnormalNodes > 0 || c.metricsUnavailable(rep):
		summary.OverallHealth = report.HealthSevere
	case len(rep.AbnormalRows) > 0 || len(rep.RestartRows) > 0:
		summary.OverallHealth = report.HealthWarn
	default:
		summary.OverallHealth = report.HealthOK
	}

	rep.Summary = summary
}

// metricsUnavailable 根据 Notes 判断是否因 metrics-server 缺失导致节点数据缺失。
func (c *Collector) metricsUnavailable(rep *report.Report) bool {
	// 节点数据为空且 Notes 含降级提示，视为 metrics 不可用
	if len(rep.NodeRows) == 0 {
		for _, note := range rep.Notes {
			if strings.Contains(note, "Metrics API 不可用") {
				return true
			}
		}
	}
	return false
}

// collectNodes 采集所有节点的资源指标。
// Metrics API 不可用时（如未安装 metrics-server）降级为提示而非崩溃。
func (c *Collector) collectNodes(ctx context.Context, rep *report.Report) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取节点列表失败: " + err.Error())
		return
	}

	// 预探测 Metrics API 是否可用，缺失时给出友好提示
	if _, err := c.metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		if isMetricsUnavailable(err) {
			rep.AddNote("无法获取节点资源指标：Metrics API 不可用。")
			rep.AddNote("这通常意味着集群未安装 metrics-server。")
			rep.AddNote("安装方法：kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
			return
		}
		rep.AddNote("查询节点指标失败: " + err.Error())
		return
	}

	failedCount := 0
	for i := range nodes.Items {
		row, err := c.collectNodeRow(ctx, &nodes.Items[i])
		if err != nil {
			log.Printf("获取节点 %s 的资源指标失败: %v", nodes.Items[i].Name, err)
			failedCount++
			continue
		}
		rep.NodeRows = append(rep.NodeRows, *row)
	}
	if failedCount > 0 {
		rep.AddNote(fmt.Sprintf("警告: %d 个节点的指标获取失败", failedCount))
	}
}

// collectPods 采集 Pod 列表（重启检查或异常检查），使用 worker pool 并发处理。
// abnormal=true 走异常分支，false 走重启分支。
func (c *Collector) collectPods(ctx context.Context, rep *report.Report, abnormal bool) {
	ns := c.cfg.Namespace
	if c.cfg.AllNamespace {
		ns = ""
	}

	pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取Pod列表失败: " + err.Error())
		return
	}
	rep.TotalPods = len(pods.Items)

	// 预加载节点信息缓存
	c.preloadNodeCache(ctx)

	workers := c.cfg.Workers
	if workers <= 0 {
		workers = 10
	}

	// worker pool：保留原并发模型，输出端从 chan []string 改为强类型 channel
	podChan := make(chan v1.Pod, len(pods.Items))
	for _, pod := range pods.Items {
		podChan <- pod
	}
	close(podChan)

	if abnormal {
		out := make(chan *report.AbnormalPodRow, len(pods.Items))
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for pod := range podChan {
					if row := c.processAbnormalPod(ctx, pod); row != nil {
						out <- row
					}
				}
			}()
		}
		wg.Wait()
		close(out)
		for row := range out {
			rep.AbnormalRows = append(rep.AbnormalRows, *row)
		}
	} else {
		out := make(chan *report.RestartPodRow, len(pods.Items))
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for pod := range podChan {
					if row := c.processRestartPod(ctx, pod); row != nil {
						out <- row
					}
				}
			}()
		}
		wg.Wait()
		close(out)
		for row := range out {
			rep.RestartRows = append(rep.RestartRows, *row)
		}
	}
}

// isMetricsUnavailable 判断错误是否表示 Metrics API 不可用（未安装 metrics-server）。
func isMetricsUnavailable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "metrics")
}

// nowInBeijing 返回当前北京时间。
func nowInBeijing() time.Time {
	loc, _ := time.LoadLocation(beijingTZ)
	return time.Now().In(loc)
}
