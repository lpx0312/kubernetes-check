package pod

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultAnalyzer_Analyze(t *testing.T) {
	analyzer := NewDefaultAnalyzer()

	tests := []struct {
		name     string
		pod      *v1.Pod
		wantNil  bool
		wantType string
	}{
		{
			name: "正常运行的Pod",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 0,
							Ready:        true,
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			wantNil: true,
		},
		{
			name: "有重启的Pod",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "restart-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 5,
							Ready:        true,
						},
					},
				},
			},
			wantType: "Restart",
		},
		{
			name: "CrashLoopBackOff",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "crash-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 10,
							Ready:        false,
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			wantType: "CrashLoopBackOff",
		},
		{
			name: "Pending状态-ContainerCreating(正常)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pending-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 0,
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{
									Reason: "ContainerCreating",
								},
							},
						},
					},
				},
			},
			wantNil: true, // ContainerCreating是正常的Pending状态
		},
		{
			name: "Pending状态-未调度",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pending-pod-unschedulable",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "",
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					Conditions: []v1.PodCondition{
						{
							Type:    v1.PodScheduled,
							Status:  v1.ConditionFalse,
							Reason:  "Unschedulable",
							Message: "0/1 nodes are available",
						},
					},
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 0,
						},
					},
				},
			},
			wantType: "NotScheduled",
		},
		{
			name: "ImagePullBackOff",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "image-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 0,
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{
									Reason:  "ImagePullBackOff",
									Message: "Back-off pulling image",
								},
							},
						},
					},
				},
			},
			wantType: "ImagePullBackOff",
		},
		{
			name: "OOMKilled",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oom-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					NodeName: "node-1",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name:         "container-1",
							RestartCount: 3,
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
					},
				},
			},
			wantType: "OOMKilled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze(tt.pod)

			if result.PodName != tt.pod.Name {
				t.Errorf("PodName = %v, want %v", result.PodName, tt.pod.Name)
			}

			if result.Namespace != tt.pod.Namespace {
				t.Errorf("Namespace = %v, want %v", result.Namespace, tt.pod.Namespace)
			}

			if tt.wantNil {
				if len(result.Issues) > 0 {
					t.Errorf("Expected no issues, got %d issues", len(result.Issues))
				}
			} else if tt.wantType != "" {
				found := false
				for _, issue := range result.Issues {
					if issue.Type == tt.wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue type %s, got %+v", tt.wantType, result.Issues)
				}
			}
		})
	}
}

func TestDefaultAnalyzer_HasIssues(t *testing.T) {
	analyzer := NewDefaultAnalyzer()

	t.Run("无问题", func(t *testing.T) {
		result := PodAnalysisResult{
			Issues: []PodIssue{},
		}
		if analyzer.HasIssues(result) {
			t.Error("Expected HasIssues to return false")
		}
	})

	t.Run("有问题", func(t *testing.T) {
		result := PodAnalysisResult{
			Issues: []PodIssue{
				{Type: "Restart"},
			},
		}
		if !analyzer.HasIssues(result) {
			t.Error("Expected HasIssues to return true")
		}
	})
}

func TestDefaultAnalyzer_GetIssueSummary(t *testing.T) {
	analyzer := NewDefaultAnalyzer()

	t.Run("无问题", func(t *testing.T) {
		result := PodAnalysisResult{
			PodName: "test-pod",
			Issues:  []PodIssue{},
		}
		summary := analyzer.GetIssueSummary(result)
		if summary != "" {
			t.Errorf("Expected empty summary, got %q", summary)
		}
	})

	t.Run("单个问题", func(t *testing.T) {
		result := PodAnalysisResult{
			PodName: "test-pod",
			Issues: []PodIssue{
				{Type: "Restart", Reason: "容器重启5次"},
			},
		}
		summary := analyzer.GetIssueSummary(result)
		if summary == "" {
			t.Error("Expected non-empty summary")
		}
	})

	t.Run("多个问题", func(t *testing.T) {
		result := PodAnalysisResult{
			PodName: "test-pod",
			Issues: []PodIssue{
				{Type: "Restart", Reason: "容器重启5次"},
				{Type: "CrashLoopBackOff", Reason: "容器崩溃"},
			},
		}
		summary := analyzer.GetIssueSummary(result)
		if summary == "" {
			t.Error("Expected non-empty summary")
		}
	})
}

func TestCalculateRestartCount(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected int32
	}{
		{
			name: "无容器",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{},
				},
			},
			expected: 0,
		},
		{
			name: "单个容器重启",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{RestartCount: 5},
					},
				},
			},
			expected: 5,
		},
		{
			name: "多个容器",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{RestartCount: 3},
						{RestartCount: 7},
					},
				},
			},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRestartCount(tt.pod)
			if result != tt.expected {
				t.Errorf("CalculateRestartCount() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetPodAge(t *testing.T) {
	t.Run("正常Pod", func(t *testing.T) {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Now(),
			},
		}
		age := GetPodAge(pod)
		if age == "" {
			t.Error("Expected non-empty age")
		}
	})
}
