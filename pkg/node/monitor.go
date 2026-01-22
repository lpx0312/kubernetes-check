package node

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// DefaultMonitor 默认的节点监控器实现
type DefaultMonitor struct{}

// NewDefaultMonitor 创建默认监控器
func NewDefaultMonitor() *DefaultMonitor {
	return &DefaultMonitor{}
}

// CollectNodeInfo 收集节点信息
func (m *DefaultMonitor) CollectNodeInfo(node *v1.Node, podCount int, problemPodCount int) NodeInfo {
	info := NodeInfo{
		Name:        node.Name,
		Ready:       m.IsNodeReady(node),
		PodCount:    podCount,
		ProblemPods: problemPodCount,
		Conditions:  m.GetNodeConditions(node),
	}

	// 获取内部IP
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			info.InternalIP = addr.Address
			break
		}
	}

	// 获取可分配资源
	info.AllocatableCPU, info.AllocatableMemory = m.GetAllocatableResources(node)

	return info
}

// IsNodeReady 检查节点是否Ready
func (m *DefaultMonitor) IsNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

// GetNodeConditions 获取节点条件列表
func (m *DefaultMonitor) GetNodeConditions(node *v1.Node) []string {
	var conditions []string

	for _, condition := range node.Status.Conditions {
		var shouldReport bool
		var condStr string

		// 对于 Ready: True 是好，False/Unknown 是坏
		// 对于压力条件: False 是好，True/Unknown 是坏
		switch condition.Type {
		case v1.NodeReady:
			// Ready 为 False 或 Unknown 时报告
			shouldReport = condition.Status != v1.ConditionTrue
			if shouldReport {
				condStr = "NotReady"
			}
		case v1.NodeMemoryPressure:
			// MemoryPressure 为 True 时报告
			shouldReport = condition.Status == v1.ConditionTrue
			if shouldReport {
				condStr = "MemoryPressure"
			}
		case v1.NodeDiskPressure:
			// DiskPressure 为 True 时报告
			shouldReport = condition.Status == v1.ConditionTrue
			if shouldReport {
				condStr = "DiskPressure"
			}
		case v1.NodePIDPressure:
			// PIDPressure 为 True 时报告
			shouldReport = condition.Status == v1.ConditionTrue
			if shouldReport {
				condStr = "PIDPressure"
			}
		case v1.NodeNetworkUnavailable:
			// NetworkUnavailable 为 True 时报告
			shouldReport = condition.Status == v1.ConditionTrue
			if shouldReport {
				condStr = "NetworkUnavailable"
			}
		default:
			// 其他条件只报告 True 的
			shouldReport = condition.Status == v1.ConditionTrue
			if shouldReport {
				condStr = string(condition.Type)
			}
		}

		if shouldReport {
			if condition.Status == v1.ConditionUnknown {
				condStr += " (Unknown)"
			}
			conditions = append(conditions, condStr)
		}
	}

	return conditions
}

// GetAllocatableResources 获取节点可分配资源
func (m *DefaultMonitor) GetAllocatableResources(node *v1.Node) (cpu, memory string) {
	if cpuQty, ok := node.Status.Allocatable[v1.ResourceCPU]; ok {
		cpu = formatResource(&cpuQty)
	}

	if memQty, ok := node.Status.Allocatable[v1.ResourceMemory]; ok {
		memory = formatResource(&memQty)
	}

	return
}

// formatResource 格式化资源数量
func formatResource(qty *resource.Quantity) string {
	if qty == nil {
		return ""
	}
	return qty.String()
}

// GetNodeInternalIP 获取节点内部IP
func GetNodeInternalIP(node *v1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

// HasNodePressure 检查节点是否有压力
func HasNodePressure(node *v1.Node) bool {
	pressureTypes := []v1.NodeConditionType{
		v1.NodeMemoryPressure,
		v1.NodeDiskPressure,
		v1.NodePIDPressure,
		v1.NodeNetworkUnavailable,
	}

	for _, condition := range node.Status.Conditions {
		for _, pressureType := range pressureTypes {
			if condition.Type == pressureType && condition.Status != v1.ConditionFalse {
				return true
			}
		}
	}

	return false
}

// GetPressureConditions 获取压力条件详情
func GetPressureConditions(node *v1.Node) []string {
	var pressures []string

	conditionMap := map[v1.NodeConditionType]string{
		v1.NodeMemoryPressure:     "MemoryPressure",
		v1.NodeDiskPressure:       "DiskPressure",
		v1.NodePIDPressure:        "PIDPressure",
		v1.NodeNetworkUnavailable: "NetworkUnavailable",
	}

	for _, condition := range node.Status.Conditions {
		if name, ok := conditionMap[condition.Type]; ok {
			if condition.Status != v1.ConditionFalse {
				status := ""
				if condition.Status == v1.ConditionUnknown {
					status = " (Unknown)"
				}
				pressures = append(pressures, name+status)
			}
		}
	}

	return pressures
}

// GetNodeStatusSummary 获取节点状态摘要
func GetNodeStatusSummary(node *v1.Node) string {
	var status []string

	// 检查Ready状态
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if condition.Status == v1.ConditionTrue {
				status = append(status, "Ready")
			} else {
				status = append(status, "NotReady")
			}
			break
		}
	}

	// 检查压力条件
	pressures := GetPressureConditions(node)
	status = append(status, pressures...)

	if len(status) == 0 {
		return "Unknown"
	}

	return strings.Join(status, ", ")
}
