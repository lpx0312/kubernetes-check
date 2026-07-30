package collector

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGetReadyStatus_验证多容器部分就绪统计（历史 bug：旧实现误报 0/n）
func TestGetReadyStatus(t *testing.T) {
	tests := []struct {
		name     string
		pod      v1.Pod
		expected string
	}{
		{
			name: "全部就绪_2个容器",
			pod: v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "a"}, {Name: "b"}}},
				Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
					{Ready: true}, {Ready: true},
				}},
			},
			expected: "2/2",
		},
		{
			name: "部分就绪_1/2",
			pod: v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "a"}, {Name: "b"}}},
				Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
					{Ready: true}, {Ready: false},
				}},
			},
			expected: "1/2",
		},
		{
			name: "全部未就绪_0/1",
			pod: v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "a"}}},
				Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
					{Ready: false},
				}},
			},
			expected: "0/1",
		},
		{
			name:     "空容器",
			pod:      v1.Pod{},
			expected: "0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getReadyStatus(tt.pod)
			if got != tt.expected {
				t.Errorf("getReadyStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestIsPodAbnormal 覆盖历史 bug：正常调度中的 Pending Pod 不应误报为异常
func TestIsPodAbnormal(t *testing.T) {
	tests := []struct {
		name     string
		pod      v1.Pod
		expected bool
	}{
		{
			name: "正常运行Pod_不异常",
			pod: v1.Pod{
				Spec: v1.PodSpec{
					NodeName:   "node-1",
					Containers: []v1.Container{{Name: "app"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{Ready: true, State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pending但容器无异常原因_不误报",
			pod: v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					ContainerStatuses: []v1.ContainerStatus{
						{Ready: false, State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{}}},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pending且ImagePullBackOff_异常",
			pod: v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					ContainerStatuses: []v1.ContainerStatus{
						{Ready: false, State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
					},
				},
			},
			expected: true,
		},
		{
			name: "Failed相位_异常",
			pod: v1.Pod{
				Status: v1.PodStatus{Phase: v1.PodFailed},
			},
			expected: true,
		},
		{
			name: "已调度但部分就绪_异常",
			pod: v1.Pod{
				Spec: v1.PodSpec{
					NodeName:   "node-1",
					Containers: []v1.Container{{Name: "a"}, {Name: "b"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{Ready: true, State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
						{Ready: false, State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{}}},
					},
				},
			},
			expected: true,
		},
		{
			name: "容器异常退出_异常",
			pod: v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{Ready: false, State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 137}}},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPodAbnormal(tt.pod)
			if got != tt.expected {
				t.Errorf("isPodAbnormal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestAnalyzeContainers 覆盖：cutoff 过滤、ExitCode=N 兜底、空状态
func TestAnalyzeContainers(t *testing.T) {
	now := time.Now().UTC()

	t.Run("无重启记录_返回零值", func(t *testing.T) {
		count, restartTime, reason := analyzeContainers(nil, 7)
		if count != 0 || reason != "" {
			t.Errorf("期望零值，得到 count=%d reason=%q", count, reason)
		}
		if !restartTime.IsZero() {
			t.Errorf("期望零时间，得到 %v", restartTime)
		}
	})

	t.Run("7天内重启_返回次数与原因", func(t *testing.T) {
		statuses := []v1.ContainerStatus{
			{
				RestartCount: 3,
				LastTerminationState: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						FinishedAt:   metav1.Time{Time: now.Add(-2 * 24 * time.Hour)},
						Reason:       "OOMKilled",
						ExitCode:     137,
					},
				},
			},
		}
		count, _, reason := analyzeContainers(statuses, 7)
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
		if reason != "OOMKilled" {
			t.Errorf("reason = %q, want OOMKilled", reason)
		}
	})

	t.Run("超出天数_返回零值", func(t *testing.T) {
		statuses := []v1.ContainerStatus{
			{
				RestartCount: 1,
				LastTerminationState: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						FinishedAt: metav1.Time{Time: now.Add(-30 * 24 * time.Hour)},
						Reason:     "OOMKilled",
					},
				},
			},
		}
		count, _, reason := analyzeContainers(statuses, 7)
		if count != 0 || reason != "" {
			t.Errorf("超出天数应过滤，得到 count=%d reason=%q", count, reason)
		}
	})

	t.Run("原因为空时兜底ExitCode", func(t *testing.T) {
		statuses := []v1.ContainerStatus{
			{
				RestartCount: 1,
				LastTerminationState: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						FinishedAt: metav1.Time{Time: now.Add(-1 * time.Hour)},
						Reason:     "", // 空原因
						ExitCode:   143,
					},
				},
			},
		}
		_, _, reason := analyzeContainers(statuses, 7)
		if reason != "ExitCode=143" {
			t.Errorf("reason = %q, want ExitCode=143", reason)
		}
	})

	t.Run("多容器取最新重启", func(t *testing.T) {
		statuses := []v1.ContainerStatus{
			{
				RestartCount: 2,
				LastTerminationState: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						FinishedAt: metav1.Time{Time: now.Add(-3 * 24 * time.Hour)},
						Reason:     "Error",
					},
				},
			},
			{
				RestartCount: 1,
				LastTerminationState: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						FinishedAt: metav1.Time{Time: now.Add(-1 * 24 * time.Hour)}, // 更新
						Reason:     "OOMKilled",
					},
				},
			},
		}
		count, _, reason := analyzeContainers(statuses, 7)
		if count != 3 {
			t.Errorf("count = %d, want 3 (2+1)", count)
		}
		if reason != "OOMKilled" {
			t.Errorf("reason = %q, want OOMKilled (最新容器的)", reason)
		}
	})
}
