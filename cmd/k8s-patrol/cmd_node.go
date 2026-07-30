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

// nodeCmd 节点资源检查子命令
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "检查节点资源使用情况",
	Long:  "检查 Kubernetes 节点的 CPU/内存使用率与 Ready 状态。\n需要集群已安装 metrics-server。",
	RunE:  runNode,
}

func init() {
	rootCmd.AddCommand(nodeCmd)
}

func runNode(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	cmd.Printf("正在检查节点资源\n")

	cfg := collector.Config{
		Kubeconfig: gKubeconfig,
		Workers:    gWorkers,
		Mode:       report.ModeNode,
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
