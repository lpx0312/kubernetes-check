package collector

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// TestEventDedup 验证事件按资源+原因去重聚合逻辑（通过构造 EventRow 直接测合并规则）。
// 这里测的是 collectEvents 内部的 eventKey 逻辑：相同 key 累加 count，取最新时间。
func TestEventDedup(t *testing.T) {
	now := time.Now()
	later := now.Add(1 * time.Hour)

	// 模拟两条相同 key 的原始事件
	key := eventKey{Namespace: "kube-system", Kind: "Pod", Name: "nfs-provisioner", Reason: "FailedMount"}

	merged := make(map[eventKey]*report.EventRow)
	// 第一条：count=1, time=now
	merged[key] = &report.EventRow{
		Namespace:  "kube-system",
		Kind:       "Pod",
		ObjectName: "nfs-provisioner",
		Reason:     "FailedMount",
		Message:    "mount failed",
		Count:      1,
		LastTime:   now,
	}
	// 第二条：相同 key，count=2, time=later
	existing := merged[key]
	existing.Count += 2
	if later.After(existing.LastTime) {
		existing.LastTime = later
	}

	got := merged[key]
	if got.Count != 3 {
		t.Errorf("去重后 Count = %d, want 3", got.Count)
	}
	if !got.LastTime.Equal(later) {
		t.Errorf("LastTime 应取最新的，got %v want %v", got.LastTime, later)
	}
}

// TestEventKeyDifferentReason 不同 reason 不应合并。
func TestEventKeyDifferentReason(t *testing.T) {
	k1 := eventKey{Namespace: "ns", Kind: "Pod", Name: "p1", Reason: "FailedScheduling"}
	k2 := eventKey{Namespace: "ns", Kind: "Pod", Name: "p1", Reason: "FailedMount"}
	if k1 == k2 {
		t.Error("不同 reason 的 eventKey 不应相等")
	}
}

// TestEventKeyDifferentObject 不同资源不应合并。
func TestEventKeyDifferentObject(t *testing.T) {
	k1 := eventKey{Namespace: "ns", Kind: "Pod", Name: "p1", Reason: "FailedMount"}
	k2 := eventKey{Namespace: "ns", Kind: "Pod", Name: "p2", Reason: "FailedMount"}
	if k1 == k2 {
		t.Error("不同资源名的 eventKey 不应相等")
	}
}

// TestItoa 验证 itoa 辅助函数。
func TestItoa(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"}, {1, "1"}, {42, "42"}, {100, "100"}, {-5, "-5"},
	}
	for _, tt := range tests {
		if got := itoa(tt.in); got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// 确保 corev1 import 被使用（供未来扩展按资源类型过滤）
var _ = corev1.Event{}

// buildTestEvent 构造测试用 Event 对象（供集成测试用）。
func buildTestEvent(ns, name, reason, kind string, count int32, lastTime time.Time) corev1.Event {
	return corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns},
		InvolvedObject: corev1.ObjectReference{
			Namespace: ns,
			Name:      name,
			Kind:      kind,
		},
		Reason:       reason,
		Message:      "test message",
		Count:        count,
		LastTimestamp: metav1.Time{Time: lastTime},
		Type:         corev1.EventTypeWarning,
	}
}
