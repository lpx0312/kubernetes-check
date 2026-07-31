package collector

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// controlPlaneComponents 需要检查的控制平面组件（按 label component 匹配）
var controlPlaneComponents = []string{
	"etcd",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

// collectControlPlane 采集控制平面组件健康状态 + apiserver /healthz 探活。
func (c *Collector) collectControlPlane(ctx context.Context, rep *report.Report) {
	// 1. 探活 apiserver /healthz
	apiHealthy := c.probeAPIServerHealth(ctx)
	if !apiHealthy {
		rep.AddNote("⚠️ apiserver /healthz 探活失败，集群可能存在严重问题")
	}

	// 2. 统计 master 节点数（作为期望实例数）
	masterCount, err := c.countMasterNodes(ctx)
	if err != nil || masterCount == 0 {
		masterCount = 1 // 兜底：探不到 master 数时按 1 处理，避免除零
		rep.AddNote("无法统计 master 节点数，控制平面期望实例数按 1 处理")
	}

	// 3. 列 kube-system 的 Pod，按 component label 分组统计
	pods, err := c.clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取 kube-system Pod 列表失败: " + err.Error())
		return
	}

	// 按组件分组
	componentPods := make(map[string][]v1.Pod)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if comp, ok := pod.Labels["component"]; ok {
			componentPods[comp] = append(componentPods[comp], *pod)
		}
	}

	// 4. 组装每个组件的行
	for _, comp := range controlPlaneComponents {
		row := buildControlPlaneRow(comp, componentPods[comp], masterCount)
		rep.ControlPlaneRows = append(rep.ControlPlaneRows, row)
	}
}

// probeAPIServerHealth 探活 apiserver /healthz，返回 "ok" 视为健康。
func (c *Collector) probeAPIServerHealth(ctx context.Context) bool {
	raw, err := c.restClient.Get().AbsPath("/healthz").DoRaw(ctx)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(raw)) == "ok"
}

// countMasterNodes 统计 control-plane 角色的节点数。
func (c *Collector) countMasterNodes(ctx context.Context) (int, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/control-plane",
	})
	if err != nil {
		return 0, err
	}
	return len(nodes.Items), nil
}

// buildControlPlaneRow 组装单个控制平面组件的健康行。
// etcd 特殊处理：若 0 个 Pod（外部 etcd/无 label），标"未检测到"不算异常。
func buildControlPlaneRow(component string, pods []v1.Pod, expected int) report.ControlPlaneRow {
	row := report.ControlPlaneRow{
		Component: component,
		Expected:  expected,
	}

	// etcd 特殊处理：0 个 Pod 不算异常
	if len(pods) == 0 {
		if component == "etcd" {
			row.Health = "未检测到"
			row.Message = "未检测到 etcd Pod（可能为外部 etcd 或静态 Pod 无 label）"
		} else {
			row.Health = "异常"
			row.Message = "未检测到 " + component + " Pod"
		}
		return row
	}

	// 统计 Ready / Abnormal
	abnormalPods := []string{}
	for i := range pods {
		pod := &pods[i]
		if !isPodReady(pod) {
			row.Abnormal++
			abnormalPods = append(abnormalPods, pod.Name)
		} else {
			row.Ready++
		}
	}

	// 健康判定
	if row.Abnormal > 0 {
		row.Health = "异常"
		row.Message = fmt.Sprintf("异常 Pod: %s", strings.Join(abnormalPods, ", "))
	} else if row.Ready < expected {
		row.Health = "异常"
		row.Message = fmt.Sprintf("Ready 数 %d < 期望 %d", row.Ready, expected)
	} else {
		row.Health = "健康"
	}
	return row
}

// isPodReady 判断 Pod 是否 Ready（所有容器 Ready 且相位 Running）。
func isPodReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	return true
}
