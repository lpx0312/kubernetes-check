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

// controlPlaneCmd 控制平面健康检查子命令
var controlPlaneCmd = &cobra.Command{
	Use:   "controlplane",
	Short: "检查控制平面健康状态",
	Long: `检查 Kubernetes 控制平面核心组件的健康状态：
  · kube-apiserver（含 /healthz 探活）
  · etcd
  · kube-scheduler
  · kube-controller-manager

通过检查 kube-system 中各组件的静态 Pod 状态判断健康度。
期望实例数取 master 节点数（control-plane 角色）。
etcd 若以外部方式部署（无 Pod），显示"未检测到"，不算异常。`,
	RunE: runControlPlane,
}

func init() {
	rootCmd.AddCommand(controlPlaneCmd)
}

func runControlPlane(cmd *cobra.Command, args []string) error {
	logKubeconfig()
	cmd.Printf("正在检查控制平面健康状态\n")

	cfg := collector.Config{
		Kubeconfig: gKubeconfig,
		Workers:    gWorkers,
		Mode:       report.ModeControlPlane,
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
