// k8s-patrol 是 Kubernetes 集群巡检 CLI（Cobra 子命令架构）。
//
// 三层架构：
//
//	cmd/k8s-patrol     - Cobra 子命令编排（本目录）
//	internal/collector - 数据采集，产出 report.Report
//	internal/report    - 巡检结果数据结构
//	internal/renderer  - 渲染层：终端表格 + HTML 报告
package main

func main() {
	Execute()
}
