package node

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultMonitor_CollectNodeInfo(t *testing.T) {
	monitor := NewDefaultMonitor()

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "192.168.1.10",
				},
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:     resource.MustParse("4"),
				v1.ResourceMemory:  resource.MustParse("8Gi"),
			},
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	info := monitor.CollectNodeInfo(node, 10, 2)

	if info.Name != "node-1" {
		t.Errorf("Name = %v, want node-1", info.Name)
	}

	if info.InternalIP != "192.168.1.10" {
		t.Errorf("InternalIP = %v, want 192.168.1.10", info.InternalIP)
	}

	if !info.Ready {
		t.Error("Ready = false, want true")
	}

	if info.PodCount != 10 {
		t.Errorf("PodCount = %v, want 10", info.PodCount)
	}

	if info.ProblemPods != 2 {
		t.Errorf("ProblemPods = %v, want 2", info.ProblemPods)
	}

	if info.AllocatableCPU != "4" {
		t.Errorf("AllocatableCPU = %v, want 4", info.AllocatableCPU)
	}

	if info.AllocatableMemory != "8Gi" {
		t.Errorf("AllocatableMemory = %v, want 8Gi", info.AllocatableMemory)
	}
}

func TestDefaultMonitor_IsNodeReady(t *testing.T) {
	monitor := NewDefaultMonitor()

	tests := []struct {
		name     string
		node     *v1.Node
		expected bool
	}{
		{
			name: "Ready节点",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:   v1.NodeReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "NotReady节点",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:   v1.NodeReady,
							Status: v1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Unknown节点",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:   v1.NodeReady,
							Status: v1.ConditionUnknown,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "无条件节点",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.IsNodeReady(tt.node)
			if result != tt.expected {
				t.Errorf("IsNodeReady() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultMonitor_GetNodeConditions(t *testing.T) {
	monitor := NewDefaultMonitor()

	node := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
				{
					Type:    v1.NodeMemoryPressure,
					Status:  v1.ConditionFalse,
					Reason:  "KubeletHasSufficientMemory",
					Message: "kubelet has sufficient memory available",
				},
				{
					Type:    v1.NodeDiskPressure,
					Status:  v1.ConditionTrue,
					Reason:  "KubeletHasNoDiskSpace",
					Message: "kubelet has no available disk space",
				},
			},
		},
	}

	conditions := monitor.GetNodeConditions(node)

	if len(conditions) == 0 {
		t.Error("Expected at least one condition")
	}

	// Check if disk pressure condition is included
	foundDiskPressure := false
	for _, cond := range conditions {
		if cond == "DiskPressure" {
			foundDiskPressure = true
			break
		}
	}
	if !foundDiskPressure {
		t.Error("Expected DiskPressure condition")
	}
}

func TestDefaultMonitor_GetAllocatableResources(t *testing.T) {
	monitor := NewDefaultMonitor()

	tests := []struct {
		name           string
		node           *v1.Node
		expectedCPU    string
		expectedMemory string
	}{
		{
			name: "正常资源",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{
						v1.ResourceCPU:     resource.MustParse("4"),
						v1.ResourceMemory:  resource.MustParse("16Gi"),
					},
				},
			},
			expectedCPU:    "4",
			expectedMemory: "16Gi",
		},
		{
			name: "无资源信息",
			node: &v1.Node{
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{},
				},
			},
			expectedCPU:    "",
			expectedMemory: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, memory := monitor.GetAllocatableResources(tt.node)
			if cpu != tt.expectedCPU {
				t.Errorf("CPU = %v, want %v", cpu, tt.expectedCPU)
			}
			if memory != tt.expectedMemory {
				t.Errorf("Memory = %v, want %v", memory, tt.expectedMemory)
			}
		})
	}
}

func TestFormatResource(t *testing.T) {
	tests := []struct {
		name     string
		parseQty string
		expected string
	}{
		{
			name:     "CPU核心",
			parseQty: "4",
			expected: "4",
		},
		{
			name:     "CPU毫核",
			parseQty: "500m",
			expected: "500m",
		},
		{
			name:     "内存GB",
			parseQty: "8Gi",
			expected: "8Gi",
		},
		{
			name:     "内存MB",
			parseQty: "512Mi",
			expected: "512Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty := resource.MustParse(tt.parseQty)
			result := formatResource(&qty)
			if result != tt.expected {
				t.Errorf("formatResource() = %v, want %v", result, tt.expected)
			}
		})
	}
}
