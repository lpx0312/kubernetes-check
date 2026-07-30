package collector

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"

	"k8s-patrol/internal/report"
)

// analyzeContainers 分析容器重启情况。
// 返回：(总重启次数, 最近重启时间, 重启原因)。
// 仅当存在 N 天内的重启事件时返回非零值，否则返回零值。
// 重启原因为空时兜底 ExitCode=N，保留历史行为。
func analyzeContainers(statuses []v1.ContainerStatus, days int) (int, time.Time, string) {
	if len(statuses) == 0 {
		return 0, time.Time{}, ""
	}

	totalRestarts := 0
	var latestRestartTime time.Time
	restartReason := ""

	for _, cs := range statuses {
		totalRestarts += int(cs.RestartCount)

		if cs.LastTerminationState.Terminated != nil {
			terminated := cs.LastTerminationState.Terminated
			if terminated.FinishedAt.IsZero() {
				continue
			}

			finishedAt := terminated.FinishedAt.Time.UTC()
			if finishedAt.After(latestRestartTime) {
				latestRestartTime = finishedAt
				restartReason = terminated.Reason
				// 原因兜底：部分 K8s 版本不填充 Reason，用退出码代替
				if restartReason == "" {
					restartReason = fmt.Sprintf("ExitCode=%d", terminated.ExitCode)
				}
			}
		}
	}

	cutoffTime := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	if totalRestarts > 0 && !latestRestartTime.IsZero() && latestRestartTime.After(cutoffTime) {
		loc, _ := time.LoadLocation(beijingTZ)
		latestRestartTime = latestRestartTime.In(loc)
		return totalRestarts, latestRestartTime, restartReason
	}
	return 0, time.Time{}, ""
}

// getReadyStatus 统计真实就绪容器数：x/y，x 为 ContainerStatuses 中 Ready=true 的数量。
// 修正历史 bug：旧实现只看 PodReady condition，导致多容器部分就绪时误报 0/n。
func getReadyStatus(pod v1.Pod) string {
	readyContainers := 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			readyContainers++
		}
	}
	totalContainers := len(pod.Spec.Containers)
	if totalContainers == 0 {
		totalContainers = len(pod.Status.ContainerStatuses)
	}
	return fmt.Sprintf("%d/%d", readyContainers, totalContainers)
}

// isPodAbnormal 判断 Pod 是否处于异常状态。
// 设计原则：避免对"正常调度中"的 Pending Pod 误报。
// 仅以下情形判为异常：
//  1. Pod 处于 Failed 或 Unknown 相位
//  2. 容器处于 Waiting（且原因非空，如 ImagePullBackOff）或异常 Terminated
//  3. 已调度 Pod 的容器未全部就绪
func isPodAbnormal(pod v1.Pod) bool {
	// Failed / Unknown 相位直接判异常
	if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodUnknown {
		return true
	}

	// 容器级异常（适用于 Pending 卡死、Running 中崩溃、Terminated 异常退出）
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return true // 如 ImagePullBackOff/ErrImagePull/CrashLoopBackOff
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return true
		}
	}

	// 对已调度的 Pod，检查就绪状态
	if pod.Spec.NodeName != "" {
		ready := getReadyStatus(pod)
		totalContainers := len(pod.Spec.Containers)
		if totalContainers == 0 {
			totalContainers = len(pod.Status.ContainerStatuses)
		}
		if ready != fmt.Sprintf("%d/%d", totalContainers, totalContainers) {
			return true
		}
	}

	return false
}

// getContainerStatusReason 提取容器状态原因，用于异常 Pod 的"容器状态"列。
func getContainerStatusReason(pod v1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil {
			return cs.State.Terminated.Reason
		}
	}
	return "N/A"
}

// processRestartPod 采集单个 Pod 的重启信息。
// 若该 Pod 在配置的天数内有重启，返回非 nil 行；否则返回 nil。
// nodeCache 用于查节点 IP，缺失时回退 API 查询并补缓存。
func (c *Collector) processRestartPod(ctx context.Context, pod v1.Pod) *report.RestartPodRow {
	restartCount, restartTime, restartReason := analyzeContainers(pod.Status.ContainerStatuses, c.cfg.Days)
	if restartCount == 0 {
		return nil
	}
	return &report.RestartPodRow{
		Namespace:    pod.Namespace,
		Name:         pod.Name,
		Phase:        string(pod.Status.Phase),
		NodeIP:       c.getNodeIPOrUnknown(ctx, pod.Spec.NodeName),
		RestartCount: restartCount,
		RestartTime:  restartTime,
		Reason:       restartReason,
		Ready:        getReadyStatus(pod),
	}
}

// processAbnormalPod 采集单个 Pod 的异常信息。
// 若该 Pod 被判定为异常，返回非 nil 行；否则返回 nil。
func (c *Collector) processAbnormalPod(ctx context.Context, pod v1.Pod) *report.AbnormalPodRow {
	if !isPodAbnormal(pod) {
		return nil
	}
	age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
	return &report.AbnormalPodRow{
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		Phase:           string(pod.Status.Phase),
		NodeIP:          c.getNodeIPOrUnknown(ctx, pod.Spec.NodeName),
		Ready:           getReadyStatus(pod),
		Age:             age,
		ContainerStatus: getContainerStatusReason(pod),
	}
}
