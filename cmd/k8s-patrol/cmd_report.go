package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s-patrol/internal/collector"
	"k8s-patrol/internal/renderer"
	"k8s-patrol/internal/report"
)

var reportOutput string

// reportCmd 全量巡检报告子命令
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "生成全量 HTML 巡检报告",
	Long: `生成一份自包含的 HTML 巡检报告，包含：
  · 巡检摘要（健康度/节点/Pod/PVC 统计）
  · 节点资源使用情况
  · 存储与卷（PVC + 孤儿 PV）
  · 异常 Pod
  · 近期重启 Pod

报告固定扫描所有命名空间。`,
	RunE: runReport,
}

func init() {
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "输出文件路径（默认 report-YYYYMMDD.html）")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	logKubeconfig()

	// 默认文件名带日期，便于每周归档
	output := reportOutput
	if output == "" {
		output = "report-" + time.Now().Format("20060102") + ".html"
	}
	cmd.Printf("正在生成全量巡检报告 → %s\n", output)

	cfg := collector.Config{
		Kubeconfig:   gKubeconfig,
		AllNamespace: true, // 全量报告固定扫所有命名空间
		Workers:      gWorkers,
		Mode:         report.ModeFull,
	}
	c, err := collector.New(cfg)
	if err != nil {
		return fmt.Errorf("创建采集器失败: %w", err)
	}
	rep, err := c.Collect(context.Background())
	if err != nil {
		return fmt.Errorf("采集失败: %w", err)
	}

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("创建报告文件失败: %w", err)
	}
	defer f.Close()
	if err := renderer.RenderHTML(rep, f); err != nil {
		return fmt.Errorf("生成HTML报告失败: %w", err)
	}
	fmt.Printf("✓ 巡检报告已生成: %s (健康度: %s)\n", output, rep.Summary.OverallHealth)
	return nil
}

// logNamespaceScope 打印命名空间查询范围提示（restart/abnormal/storage 共用）。
func logNamespaceScope(cmd *cobra.Command) {
	if gAllNamespace {
		cmd.Printf("正在查询所有命名空间\n")
	} else {
		cmd.Printf("正在查询命名空间 %s\n", gNamespace)
	}
}
