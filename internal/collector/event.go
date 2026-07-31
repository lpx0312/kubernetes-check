package collector

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// 事件展示上限，防止超大集群事件爆炸
const maxEventRows = 50

// eventKey 事件去重的聚合键：相同资源 + 相同原因视为同一事件
type eventKey struct {
	Namespace string
	Kind      string
	Name      string
	Reason    string
}

// collectEvents 采集集群 Warning 事件，按资源+原因去重聚合，按时间倒序。
// 采集失败时降级为 Note 提示，不阻塞报告。
func (c *Collector) collectEvents(ctx context.Context, rep *report.Report) {
	ns := c.cfg.Namespace
	if c.cfg.AllNamespace {
		ns = ""
	}

	// 只取 Warning 类型事件
	events, err := c.clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		rep.AddNote("获取事件列表失败: " + err.Error())
		return
	}

	// 按 eventKey 聚合去重
	merged := make(map[eventKey]*report.EventRow)
	for i := range events.Items {
		ev := &events.Items[i]
		key := eventKey{
			Namespace: ev.InvolvedObject.Namespace,
			Kind:      ev.InvolvedObject.Kind,
			Name:      ev.InvolvedObject.Name,
			Reason:    ev.Reason,
		}
		if existing, ok := merged[key]; ok {
			// 相同事件：累加次数，取最新时间
			existing.Count += ev.Count
			if ev.LastTimestamp.After(existing.LastTime) {
				existing.LastTime = ev.LastTimestamp.Time
			}
			continue
		}
		merged[key] = &report.EventRow{
			Namespace:  ev.InvolvedObject.Namespace,
			Kind:       ev.InvolvedObject.Kind,
			ObjectName: ev.InvolvedObject.Name,
			Reason:     ev.Reason,
			Message:    ev.Message,
			Count:      ev.Count,
			LastTime:   ev.LastTimestamp.Time,
		}
	}

	// 转为 slice 并按最后发生时间倒序（最新的在前）
	rows := make([]report.EventRow, 0, len(merged))
	for _, r := range merged {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].LastTime.After(rows[j].LastTime)
	})

	// 截断到上限
	if len(rows) > maxEventRows {
		rep.AddNote("⚠️ Warning 事件过多（去重后 " + itoa(len(rows)) + " 条），仅展示最近 " + itoa(maxEventRows) + " 条")
		rows = rows[:maxEventRows]
	}

	rep.EventRows = rows
}

// itoa 简单的 int→string，避免额外 import strconv 到本文件。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
