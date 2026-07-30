// Package renderer 负责把 report.Report 渲染为最终输出（当前为终端表格）。
// 它是三层架构（Collector → Report → Renderer）的消费端：
//   - 接收 *report.Report + io.Writer，与采集逻辑完全解耦
//   - 所有格式化（时间/时长）与配色集中在这一层
//   - 第二步在此新增 HTML 渲染即可，无需改动 collector
package renderer

import (
	"fmt"
	"io"
	"time"

	"github.com/olekukonko/tablewriter"

	"k8s-patrol/internal/report"
)

const (
	timeFormat = "2006-01-02 15:04:05"
	beijingTZ  = "Asia/Shanghai"
)

// Render 根据 report.Mode 选择对应表格渲染，并输出到 w。
// Notes（如 metrics-server 缺失降级提示）会先于表格输出。
func Render(rep *report.Report, w io.Writer) {
	// 先输出收集过程中的提示（如降级信息）
	for _, note := range rep.Notes {
		fmt.Fprintln(w, note)
	}

	switch rep.Mode {
	case report.ModeNode:
		renderNodes(rep, w)
	case report.ModeAbnormal:
		renderAbnormalPods(rep, w)
	case report.ModeRestart:
		renderRestartPods(rep, w)
	case report.ModeStorage:
		renderStorage(rep, w)
	default:
		renderRestartPods(rep, w)
	}
}

// renderRestartPods 渲染重启检查表格（8 列）。
func renderRestartPods(rep *report.Report, w io.Writer) {
	if len(rep.RestartRows) == 0 {
		fmt.Fprintln(w, "✓ 没有需要关注的Pod")
		return
	}
	fmt.Fprintf(w, "共处理 %d 个Pod，发现 %d 个近期重启的Pod\n", rep.TotalPods, len(rep.RestartRows))

	table := tablewriter.NewWriter(w)
	// 8列：命名空间、Pod名称、状态、节点IP、重启次数、最后重启时间、重启原因、就绪状态
	table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "重启次数", "最后重启时间", "重启原因", "就绪状态"})
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,   // Namespace
		tablewriter.ALIGN_LEFT,   // PodName
		tablewriter.ALIGN_CENTER, // PodStatus
		tablewriter.ALIGN_LEFT,   // NodeIP
		tablewriter.ALIGN_CENTER, // Restart
		tablewriter.ALIGN_LEFT,   // RestartTime
		tablewriter.ALIGN_LEFT,   // RestartReason
		tablewriter.ALIGN_CENTER, // READY
	})
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgMagentaColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiMagentaColor},
	)
	table.SetColumnColor(
		tablewriter.Colors{tablewriter.FgHiGreenColor},
		tablewriter.Colors{tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.FgHiWhiteColor},
		tablewriter.Colors{tablewriter.FgHiMagentaColor},
		tablewriter.Colors{tablewriter.FgHiYellowColor},
		tablewriter.Colors{tablewriter.FgHiBlueColor},
		tablewriter.Colors{tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.FgHiMagentaColor},
	)
	table.SetBorder(false)

	for _, r := range rep.RestartRows {
		table.Append([]string{
			r.Namespace,
			r.Name,
			r.Phase,
			r.NodeIP,
			fmt.Sprintf("%d", r.RestartCount),
			formatTime(r.RestartTime),
			r.Reason,
			r.Ready,
		})
	}
	table.Render()
}

// renderAbnormalPods 渲染异常检查表格（7 列）。
func renderAbnormalPods(rep *report.Report, w io.Writer) {
	fmt.Fprintf(w, "共处理 %d 个Pod，发现 %d 个异常Pod\n", rep.TotalPods, len(rep.AbnormalRows))
	if len(rep.AbnormalRows) == 0 {
		fmt.Fprintln(w, "✓ 没有需要关注的Pod")
		return
	}

	table := tablewriter.NewWriter(w)
	// 7列：命名空间、Pod名称、状态、节点IP、就绪状态、运行时长、容器状态
	table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "就绪状态", "运行时长", "容器状态"})
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,   // Namespace
		tablewriter.ALIGN_LEFT,   // PodName
		tablewriter.ALIGN_CENTER, // PodStatus
		tablewriter.ALIGN_LEFT,   // NodeIP
		tablewriter.ALIGN_CENTER, // READY
		tablewriter.ALIGN_RIGHT,  // AGE
		tablewriter.ALIGN_LEFT,   // ContainerStatus
	})
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgMagentaColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)
	table.SetColumnColor(
		tablewriter.Colors{tablewriter.FgHiGreenColor},
		tablewriter.Colors{tablewriter.FgHiCyanColor},
		tablewriter.Colors{tablewriter.FgHiWhiteColor},
		tablewriter.Colors{tablewriter.FgHiMagentaColor},
		tablewriter.Colors{tablewriter.FgHiYellowColor},
		tablewriter.Colors{tablewriter.FgHiBlueColor},
		tablewriter.Colors{tablewriter.FgHiCyanColor},
	)
	table.SetBorder(false)

	for _, r := range rep.AbnormalRows {
		table.Append([]string{
			r.Namespace,
			r.Name,
			r.Phase,
			r.NodeIP,
			r.Ready,
			formatDuration(r.Age),
			r.ContainerStatus,
		})
	}
	table.Render()
}

// renderNodes 渲染节点资源监控表格（9 列）。
func renderNodes(rep *report.Report, w io.Writer) {
	if len(rep.NodeRows) == 0 {
		return // Notes 已在 Render 入口输出降级提示
	}

	table := tablewriter.NewWriter(w)
	// 9列：节点名称、IP地址、CPU使用量、总CPU、CPU使用率、内存使用量、总内存、内存使用率、状态
	table.SetHeader([]string{"节点名称", "IP地址", "CPU使用量(cores)", "总CPU(cores)", "CPU使用率%", "内存使用量(Mi)", "总内存(Mi)", "内存使用率%", "状态"})
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,   // 节点名称
		tablewriter.ALIGN_LEFT,   // IP地址
		tablewriter.ALIGN_RIGHT,  // CPU使用量
		tablewriter.ALIGN_RIGHT,  // CPU总量
		tablewriter.ALIGN_RIGHT,  // CPU使用率
		tablewriter.ALIGN_RIGHT,  // 内存使用量
		tablewriter.ALIGN_RIGHT,  // 内存总量
		tablewriter.ALIGN_RIGHT,  // 内存使用率
		tablewriter.ALIGN_CENTER, // 状态
	})
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgWhiteColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgMagentaColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiMagentaColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiYellowColor},
	)
	table.SetColumnColor(
		tablewriter.Colors{tablewriter.FgHiGreenColor},   // 节点名称
		tablewriter.Colors{tablewriter.FgHiCyanColor},    // IP地址
		tablewriter.Colors{tablewriter.FgHiWhiteColor},   // CPU使用量
		tablewriter.Colors{tablewriter.FgHiMagentaColor}, // 总CPU
		tablewriter.Colors{tablewriter.FgHiYellowColor},  // CPU使用率%
		tablewriter.Colors{tablewriter.FgHiBlueColor},    // 内存使用量
		tablewriter.Colors{tablewriter.FgHiCyanColor},    // 总内存
		tablewriter.Colors{tablewriter.FgHiMagentaColor}, // 内存使用率%
		tablewriter.Colors{tablewriter.FgHiYellowColor},  // 状态
	)
	table.SetBorder(false)

	for _, r := range rep.NodeRows {
		table.Append([]string{
			r.NodeName,
			r.IP,
			fmt.Sprintf("%dm", r.CPU),
			fmt.Sprintf("%dm", r.TotalCPU),
			fmt.Sprintf("%.1f%%", r.CPUUsage),
			fmt.Sprintf("%dMi", r.Memory),
			fmt.Sprintf("%dMi", r.TotalMemory),
			fmt.Sprintf("%.1f%%", r.MemoryUsage),
			r.Status,
		})
	}
	table.Render()
}

// renderStorage 渲染存储检查表格：PVC 表（9列）+ 孤儿 PV 表（6列，有数据才显示）。
func renderStorage(rep *report.Report, w io.Writer) {
	// 无 PVC 且无孤儿 PV 时友好提示
	if len(rep.StorageRows) == 0 && len(rep.OrphanPVRows) == 0 {
		fmt.Fprintln(w, "✓ 无持久化存储使用")
		return
	}

	// PVC 表
	if len(rep.StorageRows) > 0 {
		fmt.Fprintf(w, "PVC 共 %d 个\n", len(rep.StorageRows))
		table := tablewriter.NewWriter(w)
		// 9列：命名空间、PVC名称、状态、SC、请求量、已用、使用率、PV名称、PV状态
		table.SetHeader([]string{"命名空间", "PVC名称", "状态", "StorageClass", "请求量", "已用", "使用率", "PV名称", "PV状态"})
		table.SetBorder(false)
		for _, r := range rep.StorageRows {
			usedDisplay := formatGiTerm(r.UsedGi)
			usageDisplay := formatUsageTerm(r.Mounted, r.UsagePct)
			pvPhaseDisplay := r.PVPhase
			if pvPhaseDisplay == "" {
				pvPhaseDisplay = "-"
			}
			table.Append([]string{
				r.Namespace,
				r.Name,
				r.Phase,
				r.StorageClass,
				formatGiTerm(r.RequestedGi),
				usedDisplay,
				usageDisplay,
				r.PVName,
				pvPhaseDisplay,
			})
		}
		table.Render()
	}

	// 孤儿 PV 表
	if len(rep.OrphanPVRows) > 0 {
		fmt.Fprintf(w, "\n孤儿 PV 共 %d 个（Released/Failed 且无 PVC 引用）\n", len(rep.OrphanPVRows))
		table := tablewriter.NewWriter(w)
		// 6列：PV名称、状态、回收策略、访问模式、绑定节点、容量
		table.SetHeader([]string{"PV名称", "状态", "回收策略", "访问模式", "绑定节点", "容量"})
		table.SetBorder(false)
		for _, p := range rep.OrphanPVRows {
			boundNode := p.BoundNode
			if boundNode == "" {
				boundNode = "-"
			}
			table.Append([]string{
				p.Name,
				p.Phase,
				p.ReclaimPolicy,
				p.AccessModes,
				boundNode,
				formatGiTerm(p.CapacityGi),
			})
		}
		table.Render()
	}
}

// formatGiTerm 终端版 Gi 格式化，0 显示 "-"。
func formatGiTerm(gi int64) string {
	if gi == 0 {
		return "-"
	}
	return fmt.Sprintf("%dGi", gi)
}

// formatUsageTerm 终端版使用率格式化，未挂载显示"未挂载"。
func formatUsageTerm(mounted bool, pct float64) string {
	if !mounted {
		return "未挂载"
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// formatTime 格式化时间为北京时间字符串，零值返回 "N/A"。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	loc, _ := time.LoadLocation(beijingTZ)
	return t.In(loc).Format(timeFormat) + " (UTC+8)"
}

// formatDuration 格式化时长为 "XdYh" 形式。
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	return fmt.Sprintf("%dd%dh", hours/24, hours%24)
}
