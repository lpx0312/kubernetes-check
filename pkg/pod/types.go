package pod

import (
	v1 "k8s.io/api/core/v1"
)

// PodIssue Pod问题类型
type PodIssue struct {
	Type    string // 问题类型: Restart, CrashLoop, Pending, ImagePullBackOff, etc.
	Reason  string // 具体原因
	Message string // 详细信息
}

// PodAnalysisResult Pod分析结果
type PodAnalysisResult struct {
	PodName      string
	Namespace    string
	NodeName     string
	Phase        v1.PodPhase
	RestartCount int32
	Issues       []PodIssue
	Age          string // Pod年龄
}

// PodAnalyzer Pod分析器接口
type PodAnalyzer interface {
	Analyze(pod *v1.Pod) PodAnalysisResult
	HasIssues(result PodAnalysisResult) bool
	GetIssueSummary(result PodAnalysisResult) string
}
