package main

import (
	"github.com/spf13/cobra"

	"pod-monitor/internal/log"
)

var (
	kubeconfig string
	workers    int
	verbose    bool
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:   "pod-monitor",
	Short: "Kubernetes Pod 监控和检查工具",
	Long: `Pod Monitor 是一个 Kubernetes 集群监控工具，用于检查 Pod 的重启情况、
异常状态以及节点资源使用情况。

支持的功能：
  - Pod 重启检查
  - Pod 异常状态检测
  - 节点资源使用监控

详情请访问: https://github.com/your-repo/kubernetes-check`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			log.Stdout = log.New("debug", true)
		} else if quiet {
			log.Stdout = log.New("error", true)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
		"kubeconfig 文件的绝对路径")
	rootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 10,
		"并发处理的工作协程数")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"详细输出模式")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false,
		"静默模式(只显示错误和结果)")
}
