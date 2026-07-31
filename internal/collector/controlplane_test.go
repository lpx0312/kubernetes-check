package collector

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// makeReadyPod 构造一个 Ready 的 Pod。
func makeReadyPod(name string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true},
			},
		},
	}
}

// makeNotReadyPod 构造一个未就绪的 Pod。
func makeNotReadyPod(name string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: false},
			},
		},
	}
}

// TestBuildControlPlaneRow_AllReady 全部 Ready 的组件应判健康。
func TestBuildControlPlaneRow_AllReady(t *testing.T) {
	pods := []v1.Pod{makeReadyPod("apiserver-1"), makeReadyPod("apiserver-2"), makeReadyPod("apiserver-3")}
	row := buildControlPlaneRow("kube-apiserver", pods, 3)
	if row.Health != "健康" {
		t.Errorf("Health = %q, want 健康", row.Health)
	}
	if row.Ready != 3 {
		t.Errorf("Ready = %d, want 3", row.Ready)
	}
	if row.Abnormal != 0 {
		t.Errorf("Abnormal = %d, want 0", row.Abnormal)
	}
}

// TestBuildControlPlaneRow_PartialReady 部分 Ready 应判异常。
func TestBuildControlPlaneRow_PartialReady(t *testing.T) {
	pods := []v1.Pod{makeReadyPod("scheduler-1"), makeNotReadyPod("scheduler-2"), makeReadyPod("scheduler-3")}
	row := buildControlPlaneRow("kube-scheduler", pods, 3)
	if row.Health != "异常" {
		t.Errorf("Health = %q, want 异常", row.Health)
	}
	if row.Ready != 2 {
		t.Errorf("Ready = %d, want 2", row.Ready)
	}
	if row.Abnormal != 1 {
		t.Errorf("Abnormal = %d, want 1", row.Abnormal)
	}
	if row.Message == "" {
		t.Error("异常时 Message 不应为空")
	}
}

// TestBuildControlPlaneRow_EtcdNotDetected etcd 0 个 Pod 不算异常，标"未检测到"。
func TestBuildControlPlaneRow_EtcdNotDetected(t *testing.T) {
	row := buildControlPlaneRow("etcd", []v1.Pod{}, 3)
	if row.Health != "未检测到" {
		t.Errorf("Health = %q, want 未检测到", row.Health)
	}
	if row.Message == "" {
		t.Error("未检测到时应有说明 Message")
	}
}

// TestBuildControlPlaneRow_OtherComponentMissing 非 etcd 组件 0 个 Pod 应判异常。
func TestBuildControlPlaneRow_OtherComponentMissing(t *testing.T) {
	row := buildControlPlaneRow("kube-apiserver", []v1.Pod{}, 3)
	if row.Health != "异常" {
		t.Errorf("Health = %q, want 异常", row.Health)
	}
}

// TestBuildControlPlaneRow_ReadyLessThanExpected Ready 数少于期望应判异常。
func TestBuildControlPlaneRow_ReadyLessThanExpected(t *testing.T) {
	// 期望 3，实际只有 2 个 Ready
	pods := []v1.Pod{makeReadyPod("cm-1"), makeReadyPod("cm-2")}
	row := buildControlPlaneRow("kube-controller-manager", pods, 3)
	if row.Health != "异常" {
		t.Errorf("Health = %q, want 异常（Ready<Expected）", row.Health)
	}
}

// TestIsPodReady 验证 Pod Ready 判定。
func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name string
		pod  v1.Pod
		want bool
	}{
		{"Running且容器Ready", makeReadyPod("p1"), true},
		{"Running但容器未Ready", makeNotReadyPod("p2"), false},
		{"非Running相位", v1.Pod{Status: v1.PodStatus{Phase: v1.PodPending}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodReady(&tt.pod); got != tt.want {
				t.Errorf("isPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 确保 report import 被使用（供未来扩展直接构造期望行）
var _ = report.ControlPlaneRow{}
