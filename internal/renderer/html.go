package renderer

import (
	"fmt"
	"html/template"
	"io"
	"time"

	"k8s-patrol/internal/report"
)

// 阈值常量：超出则告警，用于 CPU/内存使用率着色。
const (
	cpuWarnPct   = 80.0 // CPU 使用率 >=80% 标橙
	cpuSeverePct = 90.0 // >=90% 标红
	memWarnPct   = 85.0 // 内存 >=85% 标橙
	memSeverePct = 90.0 // >=90% 标红
)

// RenderHTML 把 Report 渲染为自包含的 HTML 巡检报告，写入 w。
// 与 Render（终端表格）平行，互不影响。
func RenderHTML(rep *report.Report, w io.Writer) error {
	data := buildHTMLData(rep)
	tmpl, err := template.New("report").Parse(reportHTMLTemplate)
	if err != nil {
		return fmt.Errorf("解析 HTML 模板失败: %w", err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("渲染 HTML 报告失败: %w", err)
	}
	return nil
}

// ---- View Model ----
// 模板只做展示，不做计算。所有格式化、阈值判定都在此处预计算成字符串/class。

type htmlReportData struct {
	Cluster     string
	GeneratedAt string
	Summary     htmlSummary
	HasNodes    bool
	NodeRows    []htmlNodeRow
	HasStorage  bool
	StorageRows []htmlStorageRow
	HasOrphanPV bool
	OrphanPVRows []htmlOrphanPVRow
	HasAbnormal bool
	AbnormalRows []htmlAbnormalRow
	HasRestart  bool
	RestartRows []htmlRestartRow
	Notes       []string
}

type htmlSummary struct {
	TotalNodes, AbnormalNodes  int
	TotalPods, AbnormalPods    int
	RestartedPods              int
	AbnormalPVC, OrphanPV      int
	OverallHealth              string
}

type htmlNodeRow struct {
	NodeName, IP, Status string
	CPUDisplay, TotalCPUDisplay string
	MemDisplay, TotalMemDisplay string
	CPUUsage, MemoryUsage float64 // 使用率，模板渲染百分比
	CPUClass, MemClass string // cell-warn / cell-severe / 空
}

type htmlAbnormalRow struct {
	Namespace, Name, Phase, NodeIP, Ready, AgeDisplay, ContainerStatus string
}

type htmlRestartRow struct {
	Namespace, Name, Phase, NodeIP string
	RestartCount                   int
	RestartTimeDisplay, Reason, Ready string
}

type htmlStorageRow struct {
	Namespace, Name, Phase, StorageClass string
	RequestedDisplay, UsedDisplay, UsageDisplay string
	UsagePct float64
	UsageClass string // cell-warn / cell-severe / 空
	PVName, PVPhase string
	PhaseClass string // PVC 状态着色
}

type htmlOrphanPVRow struct {
	Name, Phase, ReclaimPolicy, AccessModes, BoundNode string
	CapacityDisplay string
	PhaseClass string // Released/Failed 着色
}

// buildHTMLData 把 report.Report 转换为模板可用的 view model。
func buildHTMLData(rep *report.Report) htmlReportData {
	loc, _ := time.LoadLocation(beijingTZ)

	data := htmlReportData{
		Cluster:     rep.Cluster,
		GeneratedAt: rep.GeneratedAt.In(loc).Format("2006-01-02 15:04:05"),
		Notes:       rep.Notes,
		HasNodes:    len(rep.NodeRows) > 0,
		HasStorage:  len(rep.StorageRows) > 0,
		HasOrphanPV: len(rep.OrphanPVRows) > 0,
		HasAbnormal: len(rep.AbnormalRows) > 0,
		HasRestart:  len(rep.RestartRows) > 0,
	}
	data.Summary = htmlSummary{
		TotalNodes:    rep.Summary.TotalNodes,
		AbnormalNodes: rep.Summary.AbnormalNodes,
		TotalPods:     rep.Summary.TotalPods,
		AbnormalPods:  rep.Summary.AbnormalPods,
		RestartedPods: rep.Summary.RestartedPods,
		AbnormalPVC:   rep.Summary.AbnormalPVC,
		OrphanPV:      rep.Summary.OrphanPV,
		OverallHealth: rep.Summary.OverallHealth,
	}

	for _, n := range rep.NodeRows {
		data.NodeRows = append(data.NodeRows, htmlNodeRow{
			NodeName:        n.NodeName,
			IP:              n.IP,
			Status:          n.Status,
			CPUDisplay:      fmt.Sprintf("%dm", n.CPU),
			TotalCPUDisplay: fmt.Sprintf("%dm", n.TotalCPU),
			MemDisplay:      fmt.Sprintf("%dMi", n.Memory),
			TotalMemDisplay: fmt.Sprintf("%dMi", n.TotalMemory),
			CPUUsage:        n.CPUUsage,
			MemoryUsage:     n.MemoryUsage,
			CPUClass:        usageClass(n.CPUUsage, cpuWarnPct, cpuSeverePct),
			MemClass:        usageClass(n.MemoryUsage, memWarnPct, memSeverePct),
		})
	}

	for _, a := range rep.AbnormalRows {
		data.AbnormalRows = append(data.AbnormalRows, htmlAbnormalRow{
			Namespace:       a.Namespace,
			Name:            a.Name,
			Phase:           a.Phase,
			NodeIP:          a.NodeIP,
			Ready:           a.Ready,
			AgeDisplay:      formatDuration(a.Age),
			ContainerStatus: a.ContainerStatus,
		})
	}

	for _, r := range rep.RestartRows {
		data.RestartRows = append(data.RestartRows, htmlRestartRow{
			Namespace:         r.Namespace,
			Name:              r.Name,
			Phase:             r.Phase,
			NodeIP:            r.NodeIP,
			RestartCount:      r.RestartCount,
			RestartTimeDisplay: formatTimeHTML(r.RestartTime, loc),
			Reason:            r.Reason,
			Ready:             r.Ready,
		})
	}

	for _, s := range rep.StorageRows {
		row := htmlStorageRow{
			Namespace:         s.Namespace,
			Name:              s.Name,
			Phase:             s.Phase,
			StorageClass:      s.StorageClass,
			RequestedDisplay:  formatGi(s.RequestedGi),
			UsedDisplay:       formatGi(s.UsedGi),
			UsagePct:          s.UsagePct,
			UsageDisplay:      formatUsageDisplay(s.Mounted, s.UsagePct),
			UsageClass:        pvcUsageClass(s.Mounted, s.UsagePct),
			PVName:            s.PVName,
			PVPhase:           s.PVPhase,
			PhaseClass:        pvcPhaseClass(s.Phase),
		}
		data.StorageRows = append(data.StorageRows, row)
	}

	for _, p := range rep.OrphanPVRows {
		data.OrphanPVRows = append(data.OrphanPVRows, htmlOrphanPVRow{
			Name:           p.Name,
			Phase:          p.Phase,
			ReclaimPolicy:  p.ReclaimPolicy,
			AccessModes:    p.AccessModes,
			BoundNode:      boundNodeDisplay(p.BoundNode),
			CapacityDisplay: formatGi(p.CapacityGi),
			PhaseClass:     pvcPhaseClass(p.Phase),
		})
	}

	return data
}

// formatGi 把 Gi 数值格式化为展示字符串，0 显示 "-"。
func formatGi(gi int64) string {
	if gi == 0 {
		return "-"
	}
	return fmt.Sprintf("%dGi", gi)
}

// formatUsageDisplay 格式化 PVC 使用率展示：未挂载显示"未挂载"，否则显示百分比。
func formatUsageDisplay(mounted bool, pct float64) string {
	if !mounted {
		return "未挂载"
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// pvcUsageClass PVC 使用率着色：未挂载不着色，否则按阈值。
func pvcUsageClass(mounted bool, pct float64) string {
	if !mounted {
		return ""
	}
	return usageClass(pct, pvcWarnPct, pvcSeverePct)
}

// pvcPhaseClass PVC/PV 相位着色：Bound/Available 正常(绿)，Pending/Released 警告(橙)，Lost/Failed 严重(红)。
func pvcPhaseClass(phase string) string {
	switch phase {
	case "Bound", "Available":
		return "cell-ok"
	case "Pending", "Released":
		return "cell-warn"
	case "Lost", "Failed":
		return "cell-severe"
	default:
		return ""
	}
}

// boundNodeDisplay 节点亲和性展示：空显示"-"（网络存储）。
func boundNodeDisplay(node string) string {
	if node == "" {
		return "-"
	}
	return node
}

// PVC 使用率告警阈值（与 collector 对齐）
const (
	pvcWarnPct   = 85.0
	pvcSeverePct = 95.0
)

// usageClass 根据使用率返回 CSS class。
func usageClass(pct, warn, severe float64) string {
	switch {
	case pct >= severe:
		return "cell-severe"
	case pct >= warn:
		return "cell-warn"
	default:
		return ""
	}
}

// formatTimeHTML 格式化时间为字符串，零值返回 "N/A"。
func formatTimeHTML(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}
