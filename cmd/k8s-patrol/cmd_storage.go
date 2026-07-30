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

// storageCmd 存储检查子命令：查 PVC 绑定状态与使用量
var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "检查 PVC 绑定状态与使用量",
	Long: `检查持久化存储的使用情况，包括：
  · PVC 绑定状态（Bound/Pending/Lost）
  · PVC 使用量与使用率（需 kubelet /stats/summary 可达）
  · 孤儿 PV（Released/Failed 且无 PVC 引用）

注意：未挂载的 PVC（当前无 Pod 使用）显示"未挂载"，
其卷内可能仍有数据，使用量暂不可知，并非一定为空。`,
	RunE: runStorage,
}

func init() {
	rootCmd.AddCommand(storageCmd)
}

func runStorage(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	logNamespaceScope(cmd)

	cfg := collector.Config{
		Kubeconfig:   gKubeconfig,
		Namespace:    gNamespace,
		AllNamespace: gAllNamespace,
		Workers:      gWorkers,
		Mode:         report.ModeStorage,
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
