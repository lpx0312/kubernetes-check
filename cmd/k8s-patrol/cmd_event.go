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

// eventCmd 事件检查子命令：查集群 Warning 事件
var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "检查集群 Warning 事件",
	Long: `采集集群 Warning 事件，按资源+原因去重后展示。
可用于定位 Pod 异常根因，如：
  · 调度失败（FailedScheduling / Insufficient memory）
  · 镜像拉取失败（Failed / ImagePullBackOff）
  · 挂载失败（FailedMount / Unable to attach volume）

事件按最后发生时间倒序，最多展示 50 类。`,
	RunE: runEvent,
}

func init() {
	rootCmd.AddCommand(eventCmd)
}

func runEvent(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	logNamespaceScope(cmd)

	cfg := collector.Config{
		Kubeconfig:   gKubeconfig,
		Namespace:    gNamespace,
		AllNamespace: gAllNamespace,
		Workers:      gWorkers,
		Mode:         report.ModeEvent,
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
