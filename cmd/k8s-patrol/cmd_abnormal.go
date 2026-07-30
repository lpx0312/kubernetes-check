package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k8s-patrol/internal/collector"
	"k8s-patrol/internal/renderer"
	"k8s-patrol/internal/report"
)

// abnormalCmd 异常检查子命令：查当前状态异常的 Pod
var abnormalCmd = &cobra.Command{
	Use:   "abnormal",
	Short: "检查当前异常的 Pod",
	Long: `检查当前状态异常的 Pod，包括：
  · Pending/Failed/Unknown 相位的 Pod
  · 容器处于 CrashLoopBackOff/ImagePullBackOff 等异常状态
  · 容器未全部就绪（如 0/1、1/2）`,
	RunE: runAbnormal,
}

func init() {
	rootCmd.AddCommand(abnormalCmd)
}

func runAbnormal(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	logNamespaceScope(cmd)

	cfg := collector.Config{
		Kubeconfig:   gKubeconfig,
		Namespace:    gNamespace,
		AllNamespace: gAllNamespace,
		Workers:      gWorkers,
		Mode:         report.ModeAbnormal,
	}
	c, err := collector.New(cfg)
	if err != nil {
		return fmt.Errorf("创建采集器失败: %w", err)
	}
	rep, err := c.Collect(context.Background())
	if err != nil {
		return fmt.Errorf("采集失败: %w", err)
	}
	renderer.Render(rep, os.Stdout)
	return nil
}
