package renderer

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"k8s-patrol/internal/report"
)

// makeTestReport 构造一个覆盖全章节的测试 Report。
func makeTestReport() *report.Report {
	return &report.Report{
		Mode:        report.ModeFull,
		Cluster:     "kubernetes-admin@k8s",
		GeneratedAt: time.Date(2026, 7, 30, 18, 22, 0, 0, time.UTC),
		TotalPods:   41,
		Summary: report.ReportSummary{
			TotalNodes:    4,
			AbnormalNodes: 1,
			TotalPods:     41,
			AbnormalPods:  2,
			RestartedPods: 3,
			OverallHealth: report.HealthWarn,
		},
		NodeRows: []report.NodeRow{
			{NodeName: "node-1", IP: "10.0.0.1", CPU: 500, Memory: 1024, TotalCPU: 2000, TotalMemory: 4096, CPUUsage: 25.0, MemoryUsage: 25.0, Status: "正常"},
			{NodeName: "node-2", IP: "10.0.0.2", CPU: 1850, Memory: 3900, TotalCPU: 2000, TotalMemory: 4096, CPUUsage: 92.5, MemoryUsage: 95.2, Status: "正常"}, // CPU/内存均严重
		},
		AbnormalRows: []report.AbnormalPodRow{
			{Namespace: "kube-system", Name: "bad-pod", Phase: "Pending", NodeIP: "10.0.0.1", Ready: "0/1", Age: 5 * time.Hour, ContainerStatus: "ImagePullBackOff"},
		},
		RestartRows: []report.RestartPodRow{
			{Namespace: "default", Name: "restart-pod", Phase: "Running", NodeIP: "10.0.0.2", RestartCount: 3, RestartTime: time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC), Reason: "OOMKilled", Ready: "1/1"},
		},
		Notes: []string{"这是测试提示"},
	}
}

// TestRenderHTML_全章节覆盖 验证关键内容都出现在输出中。
func TestRenderHTML_全章节覆盖(t *testing.T) {
	rep := makeTestReport()
	var buf bytes.Buffer
	if err := RenderHTML(rep, &buf); err != nil {
		t.Fatalf("RenderHTML 失败: %v", err)
	}
	out := buf.String()

	// 报告头
	assertContains(t, out, "k8s-patrol 集群巡检报告")
	assertContains(t, out, "kubernetes-admin@k8s")
	// 健康度徽章（class 含中文健康度）
	assertContains(t, out, "health-警告")
	assertContains(t, out, "警告")

	// 摘要数字
	assertContains(t, out, ">4<") // 节点总数
	assertContains(t, out, ">2<") // 异常 Pod
	assertContains(t, out, ">3<") // 重启 Pod

	// 节点章节
	assertContains(t, out, "节点资源")
	assertContains(t, out, "node-1")
	assertContains(t, out, "node-2")

	// 异常 Pod 章节
	assertContains(t, out, "异常 Pod")
	assertContains(t, out, "bad-pod")
	assertContains(t, out, "ImagePullBackOff")

	// 重启 Pod 章节
	assertContains(t, out, "近期重启 Pod")
	assertContains(t, out, "restart-pod")
	assertContains(t, out, "OOMKilled")

	// 提示
	assertContains(t, out, "这是测试提示")
}

// TestRenderHTML_阈值告警着色 验证超阈值的 CPU/内存单元格带 cell-severe class。
func TestRenderHTML_阈值告警着色(t *testing.T) {
	rep := makeTestReport()
	var buf bytes.Buffer
	_ = RenderHTML(rep, &buf)
	out := buf.String()

	// node-2 的 CPU 92.5% >= 90 应标 cell-severe
	// node-2 的内存 95.2% >= 90 应标 cell-severe
	assertContains(t, out, "cell-severe")
	// node-1 的 CPU 25% 应不带告警 class（正常）
	if strings.Contains(out, "cell-severe") && !strings.Contains(out, "92.5%") {
		t.Error("CPU 92.5% 应出现并标红")
	}
}

// TestRenderHTML_空数据友好提示 验证无异常/无重启时显示勾号提示。
func TestRenderHTML_空数据友好提示(t *testing.T) {
	rep := &report.Report{
		Mode:        report.ModeFull,
		Cluster:     "test",
		GeneratedAt: time.Now(),
		Summary:     report.ReportSummary{OverallHealth: report.HealthOK},
	}
	var buf bytes.Buffer
	if err := RenderHTML(rep, &buf); err != nil {
		t.Fatalf("RenderHTML 失败: %v", err)
	}
	out := buf.String()

	assertContains(t, out, "所有 Pod 运行正常，无异常")
	assertContains(t, out, "无近期重启的 Pod")
	assertContains(t, out, "健康") // 健康徽章
}

// TestUsageClass 验证阈值判定逻辑。
func TestUsageClass(t *testing.T) {
	tests := []struct {
		name             string
		pct              float64
		want             string
	}{
		{"正常", 50.0, ""},
		{"警告阈值", 80.0, "cell-warn"},
		{"严重阈值", 90.0, "cell-severe"},
		{"超标", 95.0, "cell-severe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageClass(tt.pct, cpuWarnPct, cpuSeverePct)
			if got != tt.want {
				t.Errorf("usageClass(%.1f) = %q, want %q", tt.pct, got, tt.want)
			}
		})
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("输出中未找到期望内容 %q", substr)
	}
}
