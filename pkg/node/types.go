package node

import (
	v1 "k8s.io/api/core/v1"
)

// NodeInfo 节点信息
type NodeInfo struct {
	Name              string
	InternalIP        string
	Ready             bool
	AllocatableCPU    string
	AllocatableMemory string
	PodCount          int
	ProblemPods       int
	Conditions        []string
}

// NodeMonitor 节点监控器接口
type NodeMonitor interface {
	CollectNodeInfo(node *v1.Node, podCount int, problemPodCount int) NodeInfo
	IsNodeReady(node *v1.Node) bool
	GetNodeConditions(node *v1.Node) []string
	GetAllocatableResources(node *v1.Node) (cpu, memory string)
}
