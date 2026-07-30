package main

import (
	"os"

	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd"
)

// 全局选项（所有子命令通过 PersistentFlags 继承）
var (
	gKubeconfig   string
	gAllNamespace bool
	gNamespace    string
	gWorkers      int
)

// 版本信息（用 var 而非 const，以便 go build -ldflags -X 在发布时注入版本号）
// 默认值用于本地开发构建，CI/Release 流水线通过 LDFLAGS 覆盖。
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "k8s-patrol",
	Short: "Kubernetes 集群巡检工具",
	Long: `k8s-patrol 是一个 Kubernetes 集群巡检工具，支持检查：
  · 节点资源使用情况（CPU/内存）
  · Pod 重启与异常状态
  · PVC 绑定状态与使用量
  · 生成全量 HTML 巡检报告`,
	SilenceUsage: true,
}

// versionCmd 版本子命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("k8s-patrol %s\n", version)
		cmd.Printf("  commit:     %s\n", commit)
		cmd.Printf("  build date: %s\n", buildDate)
		cmd.Println("  Kubernetes 1.31.x compatible")
	},
}

func init() {
	// 全局持久选项（所有子命令继承）
	rootCmd.PersistentFlags().StringVarP(&gKubeconfig, "kubeconfig", "k", "",
		"kubeconfig 文件路径")
	rootCmd.PersistentFlags().BoolVarP(&gAllNamespace, "all-namespaces", "A", false,
		"查询所有命名空间")
	rootCmd.PersistentFlags().StringVarP(&gNamespace, "namespace", "n", "default",
		"指定命名空间")
	rootCmd.PersistentFlags().IntVarP(&gWorkers, "workers", "w", 10,
		"并发处理数")

	rootCmd.AddCommand(versionCmd)
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// logKubeconfig 打印 kubeconfig 路径提示（保持历史日志风格）。
func logKubeconfig() {
	if gKubeconfig != "" {
		rootCmd.Printf("使用自定义kubeconfig: %s\n", gKubeconfig)
	} else {
		rootCmd.Printf("使用默认kubeconfig路径: %s\n", clientcmd.RecommendedHomeFile)
	}
}
