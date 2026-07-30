// k8s-patrol 是 Kubernetes 集群巡检 CLI。
//
// 三层架构：
//   cmd/k8s-patrol     - flag 解析与编排（本文件）
//   internal/collector - 数据采集，产出 report.Report
//   internal/report    - 巡检结果数据结构
//   internal/renderer  - 把 Report 渲染为终端表格（第二步将增加 HTML）
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"k8s.io/client-go/tools/clientcmd"

	"k8s-patrol/internal/collector"
	"k8s-patrol/internal/renderer"
	"k8s-patrol/internal/report"
)

func main() {
	// 命令行参数解析
	versionFlag := flag.Bool("version", false, "显示版本信息")
	kubeconfigFlag := flag.String("kubeconfig", "", "绝对路径到kubeconfig文件")
	daysFlag := flag.Int("days", 7, "显示最近N天内重启的Pod (默认7天)")
	workersFlag := flag.Int("workers", 10, "并发处理的工作协程数 (默认10)")
	abnormalFlag := flag.Bool("abnormal", false, "仅显示异常状态Pod")
	allNamespaces := flag.Bool("A", false, "查看所有命名空间的Pod")
	namespace := flag.String("n", "default", "指定要查看的命名空间")
	nodeMetricsFlag := flag.Bool("node-metrics", false, "显示节点资源使用情况")
	flag.Parse()

	if *versionFlag {
		fmt.Println("k8s-patrol v1.1 (Kubernetes 1.31.x compatible)")
		return
	}

	// kubeconfig 路径提示（保持历史日志输出）
	if *kubeconfigFlag != "" {
		log.Printf("使用自定义kubeconfig: %s", *kubeconfigFlag)
	} else {
		log.Printf("使用默认kubeconfig路径: %s", clientcmd.RecommendedHomeFile)
	}

	// 确定 Mode 与命名空间
	mode := report.ModeRestart
	if *nodeMetricsFlag {
		mode = report.ModeNode
	} else if *abnormalFlag {
		mode = report.ModeAbnormal
	}
	if mode != report.ModeNode {
		if *allNamespaces {
			log.Printf("正在查看所有命名空间的Pod")
		} else {
			log.Printf("正在查看命名空间 %s 的Pod", *namespace)
		}
	}

	// 构造采集配置（消除历史全局 flag 指针泄漏）
	cfg := collector.Config{
		Kubeconfig:   *kubeconfigFlag,
		Namespace:    *namespace,
		AllNamespace: *allNamespaces,
		Days:         *daysFlag,
		Workers:      *workersFlag,
		Mode:         mode,
	}

	// 构造采集器
	c, err := collector.New(cfg)
	if err != nil {
		log.Fatalf("创建采集器失败: %v", err)
	}

	// 采集 → 渲染
	ctx := context.Background()
	rep, err := c.Collect(ctx)
	if err != nil {
		log.Fatalf("采集失败: %v", err)
	}
	renderer.Render(rep, os.Stdout)
}
