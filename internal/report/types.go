// Package report 定义巡检报告的数据结构。
// Collector 层产出 Report，Renderer 层消费 Report，二者通过本包解耦。
package report

import "time"

// Mode 标识本次巡检的检查模式，决定 Report 渲染哪类行。
const (
	ModeRestart  = "restart"  // 重启检查：展示 N 天内重启的 Pod
	ModeAbnormal = "abnormal" // 异常检查：展示状态异常的 Pod
	ModeNode     = "node"     // 节点监控：展示节点资源使用情况
	ModeFull     = "full"     // 全量巡检：节点 + 异常 + 重启，用于生成完整报告
)

// 健康度等级常量，供 ReportSummary.OverallHealth 使用。
const (
	HealthOK     = "健康" // 异常Pod=0 且 重启Pod=0 且 异常节点=0
	HealthWarn   = "警告" // 存在异常Pod 或 重启Pod>0
	HealthSevere = "严重" // 存在异常节点(NotReady) 或 metrics-server 缺失
)

// RestartPodRow 重启检查模式下的一行结果。
// 字段保留原始类型（time.Time），格式化职责下沉到 Renderer。
type RestartPodRow struct {
	Namespace    string
	Name         string
	Phase        string
	NodeIP       string
	RestartCount int
	RestartTime  time.Time // 最近一次重启时间（已转北京时间）
	Reason       string    // 重启原因，为空时由 Collector 兜底 ExitCode=N
	Ready        string    // 就绪状态，如 "1/1"
}

// AbnormalPodRow 异常检查模式下的一行结果。
type AbnormalPodRow struct {
	Namespace       string
	Name            string
	Phase           string
	NodeIP          string
	Ready           string
	Age             time.Duration // Pod 运行时长（原始，格式化下沉 Renderer）
	ContainerStatus string        // 容器状态原因，如 ImagePullBackOff / CrashLoopBackOff
}

// NodeRow 节点资源监控的一行结果。
// CPU 单位为毫核(m)，内存单位为 Mi，总量统一取 Allocatable。
type NodeRow struct {
	NodeName    string
	IP          string
	CPU         int64 // 毫核
	Memory      int64 // Mi
	TotalCPU    int64 // 毫核
	TotalMemory int64 // Mi
	CPUUsage    float64
	MemoryUsage float64
	Status      string // "正常" / "异常"
}

// ReportSummary 巡检摘要，由全量模式下 Collector 计算，渲染到报告顶部。
type ReportSummary struct {
	TotalNodes    int
	AbnormalNodes int // NotReady 或指标获取失败的节点数
	TotalPods     int
	AbnormalPods  int
	RestartedPods int
	OverallHealth string // Health* 常量之一
}

// Report 一次巡检的完整结果，是 Collector 与 Renderer 之间的契约。
type Report struct {
	Mode        string // 见 Mode* 常量
	Cluster     string // kubeconfig 当前 context 名，报告头展示用
	GeneratedAt time.Time
	TotalPods   int // 参与本次巡检的 Pod 总数

	RestartRows  []RestartPodRow
	AbnormalRows []AbnormalPodRow
	NodeRows     []NodeRow

	// Summary 全量模式下的巡检摘要，由 computeSummary 填充。
	// 非全量模式（终端表格）下为零值，不影响渲染。
	Summary ReportSummary

	// Notes 收集过程中的提示信息（如 metrics-server 未安装的降级提示），
	// Renderer 会将其与表格一起输出。
	Notes []string
}

// AddNote 追加一条提示信息。
func (r *Report) AddNote(note string) {
	r.Notes = append(r.Notes, note)
}
