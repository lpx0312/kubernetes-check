package collector

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// preloadNodeCache 预加载所有节点信息到缓存（节点名 → 内网 IP）。
// 避免在处理每个 Pod 时重复查询节点 API，提升大规模集群性能。
func (c *Collector) preloadNodeCache(ctx context.Context) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return // 预加载失败不阻塞，后续按需回退查询
	}
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				c.nodeCache.Store(node.Name, addr.Address)
				break
			}
		}
	}
}

// getNodeIP 从缓存获取节点 IP；缓存未命中时回退查询 API 并补缓存。
// nodeCache 类型为 sync.Map，在 worker pool 并发场景下安全。
func (c *Collector) getNodeIP(ctx context.Context, nodeName string) string {
	if nodeName == "" {
		return "N/A"
	}
	if cached, ok := c.nodeCache.Load(nodeName); ok {
		if ip, ok := cached.(string); ok {
			return ip
		}
	}
	// 缓存未命中，回退查询
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "Unknown"
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			c.nodeCache.Store(nodeName, addr.Address)
			return addr.Address
		}
	}
	return "N/A"
}

// getNodeIPOrUnknown 获取节点 IP，未知时返回 "Unknown"。
// 用于 Pod 行的节点 IP 展示。
func (c *Collector) getNodeIPOrUnknown(ctx context.Context, nodeName string) string {
	ip := c.getNodeIP(ctx, nodeName)
	if ip == "" || ip == "N/A" {
		return "Unknown"
	}
	return ip
}

// collectNodeRow 采集单个节点的资源指标，返回报告行。
// 复用传入的 node 对象提取 IP 与状态，避免 N+1 查询。
// CPU/内存总量统一使用 Allocatable，口径一致。
// 当 Metrics API 对该节点返回 not found（如 kubelet rootFs 统计坏掉、metrics-server 没抓到），
// 返回降级行（指标为零值、状态标"指标不可用"），而非丢弃该节点——
// 这样报告里能明确显示"这个节点有问题"，而不是悄悄消失。
func (c *Collector) collectNodeRow(ctx context.Context, node *v1.Node) (*report.NodeRow, error) {
	nodeMetrics, err := c.metrics.MetricsV1beta1().NodeMetricses().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		return c.degradedNodeRow(ctx, node, err)
	}

	cpuUsage := nodeMetrics.Usage.Cpu().MilliValue()
	memoryUsage := nodeMetrics.Usage.Memory().Value() / (1024 * 1024)
	// 统一使用 Allocatable，反映真实可分配资源
	totalCPU := node.Status.Allocatable.Cpu().MilliValue()
	totalMemory := node.Status.Allocatable.Memory().Value() / (1024 * 1024)

	cpuUsagePercent := float64(cpuUsage) / float64(totalCPU) * 100
	memoryUsagePercent := float64(memoryUsage) / float64(totalMemory) * 100

	// 节点 Ready 状态
	status := "正常"
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			status = "异常"
			break
		}
	}

	// 从已有 node 对象提取 IP，不再额外 API 调用
	ip := "N/A"
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			ip = addr.Address
			break
		}
	}

	return &report.NodeRow{
		NodeName:    node.Name,
		IP:          ip,
		CPU:         cpuUsage,
		Memory:      memoryUsage,
		TotalCPU:    totalCPU,
		TotalMemory: totalMemory,
		CPUUsage:    cpuUsagePercent,
		MemoryUsage: memoryUsagePercent,
		Status:      status,
	}, nil
}

// degradedNodeRow 在节点指标获取失败时，返回一个占位行。
// 关键设计：节点名/IP/Ready 状态仍从 Node 对象正常提取（这些不依赖 metrics-server），
// 只是 CPU/内存使用量无法获取，状态标"指标不可用"以醒目提示。
// 这样报告里会明确出现这个节点，而不是静默丢失。
func (c *Collector) degradedNodeRow(ctx context.Context, node *v1.Node, metricsErr error) (*report.NodeRow, error) {
	// Ready 状态仍可从 Node 对象正常判断（不依赖 metrics-server）
	status := "指标不可用"
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			status = "异常(指标不可用)"
			break
		}
	}

	ip := "N/A"
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			ip = addr.Address
			break
		}
	}

	// 总量仍从 Allocatable 取（可用），使用量为 0
	totalCPU := node.Status.Allocatable.Cpu().MilliValue()
	totalMemory := node.Status.Allocatable.Memory().Value() / (1024 * 1024)

	return &report.NodeRow{
		NodeName:    node.Name,
		IP:          ip,
		CPU:         0,
		Memory:      0,
		TotalCPU:    totalCPU,
		TotalMemory: totalMemory,
		CPUUsage:    0,
		MemoryUsage: 0,
		Status:      status,
	}, nil
}
