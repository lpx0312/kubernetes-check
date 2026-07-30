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

var (
	restartDays int
)

// restartCmd 重启检查子命令：查近 N 天内重启过的 Pod
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "检查近期重启的 Pod",
	Long:  "检查近期（默认7天内）发生过重启的 Pod，展示重启次数、时间与原因。",
	RunE:  runRestart,
}

func init() {
	restartCmd.Flags().IntVarP(&restartDays, "days", "d", 7, "回溯天数（默认7天）")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	logNamespaceScope(cmd)

	cfg := collector.Config{
		Kubeconfig:   gKubeconfig,
		Namespace:    gNamespace,
		AllNamespace: gAllNamespace,
		Workers:      gWorkers,
		Days:         restartDays,
		Mode:         report.ModeRestart,
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
