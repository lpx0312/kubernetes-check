package pod

import (
	"strings"

	v1 "k8s.io/api/core/v1"
)

// DefaultAnalyzer 默认的Pod分析器实现
type DefaultAnalyzer struct{}

// NewDefaultAnalyzer 创建默认分析器
func NewDefaultAnalyzer() *DefaultAnalyzer {
	return &DefaultAnalyzer{}
}

// Analyze 分析Pod状态并返回分析结果
func (a *DefaultAnalyzer) Analyze(pod *v1.Pod) PodAnalysisResult {
	result := PodAnalysisResult{
		PodName:      pod.Name,
		Namespace:    pod.Namespace,
		NodeName:     pod.Spec.NodeName,
		Phase:        pod.Status.Phase,
		RestartCount: CalculateRestartCount(pod),
		Age:          GetPodAge(pod),
		Issues:       []PodIssue{},
	}

	// 检查容器状态
	for _, cs := range pod.Status.ContainerStatuses {
		// 检查等待状态
		if cs.State.Waiting != nil {
			a.checkWaitingState(cs, &result)
		}

		// 检查终止状态
		if cs.LastTerminationState.Terminated != nil {
			a.checkTerminatedState(cs, &result)
		}

		// 检查重启次数
		if cs.RestartCount > 0 {
			a.checkRestarts(cs, &result)
		}
	}

	// 检查Pod Phase
	a.checkPodPhase(pod, &result)

	// 检查Pod条件
	a.checkPodConditions(pod, &result)

	return result
}

// checkWaitingState 检查容器等待状态
func (a *DefaultAnalyzer) checkWaitingState(cs v1.ContainerStatus, result *PodAnalysisResult) {
	reason := cs.State.Waiting.Reason
	message := cs.State.Waiting.Message

	switch reason {
	case "CrashLoopBackOff":
		result.Issues = append(result.Issues, PodIssue{
			Type:    "CrashLoopBackOff",
			Reason:  "容器持续崩溃",
			Message: message,
		})
	case "ImagePullBackOff", "ErrImagePull":
		result.Issues = append(result.Issues, PodIssue{
			Type:    "ImagePullBackOff",
			Reason:  "镜像拉取失败",
			Message: message,
		})
	case "ContainerCreating":
		// 正常状态，不需要添加问题
	case "ErrImageNeverPull":
		result.Issues = append(result.Issues, PodIssue{
			Type:    "ImageError",
			Reason:  "镜像配置错误",
			Message: message,
		})
	default:
		if reason != "" && reason != "ContainerCreating" {
			result.Issues = append(result.Issues, PodIssue{
				Type:    "Waiting",
				Reason:  reason,
				Message: message,
			})
		}
	}
}

// checkTerminatedState 检查容器终止状态
func (a *DefaultAnalyzer) checkTerminatedState(cs v1.ContainerStatus, result *PodAnalysisResult) {
	terminated := cs.LastTerminationState.Terminated

	switch terminated.Reason {
	case "OOMKilled":
		result.Issues = append(result.Issues, PodIssue{
			Type:    "OOMKilled",
			Reason:  "内存溢出",
			Message: terminated.Message,
		})
	case "Error":
		if cs.RestartCount > 3 {
			result.Issues = append(result.Issues, PodIssue{
				Type:    "ContainerError",
				Reason:  "容器错误退出",
				Message: terminated.Message,
			})
		}
	case "Completed":
		// Job完成的正常状态
	default:
		if terminated.ExitCode != 0 {
			result.Issues = append(result.Issues, PodIssue{
				Type:    "Terminated",
				Reason:  terminated.Reason,
				Message: terminated.Message,
			})
		}
	}
}

// checkRestarts 检查重启情况
func (a *DefaultAnalyzer) checkRestarts(cs v1.ContainerStatus, result *PodAnalysisResult) {
	if IsHighRestart(cs.RestartCount) {
		// 避免重复添加CrashLoopBackOff
		hasCrashLoop := false
		for _, issue := range result.Issues {
			if issue.Type == "CrashLoopBackOff" {
				hasCrashLoop = true
				break
			}
		}

		if !hasCrashLoop {
			result.Issues = append(result.Issues, PodIssue{
				Type:    "Restart",
				Reason:  "容器频繁重启",
				Message: cs.Name + " 已重启",
			})
		}
	}
}

// checkPodPhase 检查Pod阶段
func (a *DefaultAnalyzer) checkPodPhase(pod *v1.Pod, result *PodAnalysisResult) {
	switch pod.Status.Phase {
	case v1.PodFailed:
		// 检查是否已经记录了具体问题
		if len(result.Issues) == 0 {
			result.Issues = append(result.Issues, PodIssue{
				Type:    "Failed",
				Reason:  "Pod失败",
				Message: pod.Status.Message,
			})
		}
	case v1.PodPending:
		// 检查是否有调度问题
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodScheduled && condition.Status == v1.ConditionFalse {
				result.Issues = append(result.Issues, PodIssue{
					Type:    "NotScheduled",
					Reason:  "Pod未调度",
					Message: condition.Message,
				})
				break
			}
		}
	}
}

// checkPodConditions 检查Pod条件
func (a *DefaultAnalyzer) checkPodConditions(pod *v1.Pod, result *PodAnalysisResult) {
	for _, condition := range pod.Status.Conditions {
		// 检查Ready状态
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionFalse {
			// 只有在Running状态时才标记为NotReady
			if pod.Status.Phase == v1.PodRunning {
				alreadyNoted := false
				for _, issue := range result.Issues {
					if issue.Type == "NotReady" {
						alreadyNoted = true
						break
					}
				}
				if !alreadyNoted && len(result.Issues) == 0 {
					result.Issues = append(result.Issues, PodIssue{
						Type:    "NotReady",
						Reason:  "Pod未就绪",
						Message: condition.Message,
					})
				}
			}
		}
	}
}

// HasIssues 检查分析结果是否有问题
func (a *DefaultAnalyzer) HasIssues(result PodAnalysisResult) bool {
	return len(result.Issues) > 0
}

// GetIssueSummary 获取问题摘要
func (a *DefaultAnalyzer) GetIssueSummary(result PodAnalysisResult) string {
	if !a.HasIssues(result) {
		return ""
	}

	var summaries []string
	for _, issue := range result.Issues {
		summaries = append(summaries, issue.Type+": "+issue.Reason)
	}

	return strings.Join(summaries, "; ")
}
