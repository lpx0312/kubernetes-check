package pod

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
)

// CalculateRestartCount 计算Pod的总重启次数
func CalculateRestartCount(pod *v1.Pod) int32 {
	var count int32 = 0
	for _, cs := range pod.Status.ContainerStatuses {
		count += cs.RestartCount
	}
	return count
}

// GetPodAge 获取Pod的年龄
func GetPodAge(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}

	age := time.Since(pod.CreationTimestamp.Time)
	if age < time.Minute {
		return fmt.Sprintf("%ds", int(age.Seconds()))
	} else if age < time.Hour {
		return fmt.Sprintf("%dm", int(age.Minutes()))
	} else if age < 24*time.Hour {
		return fmt.Sprintf("%dh", int(age.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(age.Hours()/24))
	}
}

// IsHighRestart 检查是否有高重启次数
func IsHighRestart(count int32) bool {
	return count >= 5
}

// IsCrashLoopBackOff 检查是否是CrashLoopBackOff
func IsCrashLoopBackOff(cs v1.ContainerStatus) bool {
	if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
		return true
	}
	if cs.LastTerminationState.Terminated != nil &&
		cs.LastTerminationState.Terminated.Reason == "Error" &&
		cs.RestartCount > 0 {
		return true
	}
	return false
}
