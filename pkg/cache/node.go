package cache

import (
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"pod-monitor/internal/log"
	"pod-monitor/pkg/k8s"
)

// NodeCache 节点信息缓存
type NodeCache struct {
	sync.Map
}

// PreloadNodes 预加载所有节点信息到缓存
func (nc *NodeCache) PreloadNodes(ctx context.Context, client *k8s.Client) error {
	log.Stdout.DebugContext(ctx, "开始预加载节点信息")

	nodes, err := client.Clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Stdout.ErrorContext(ctx, "获取节点列表失败", "error", err)
		return err
	}

	count := 0
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				nc.Store(node.Name, addr.Address)
				count++
				break
			}
		}
	}

	log.Stdout.InfoContext(ctx, "节点信息预加载完成", "total", len(nodes.Items), "cached", count)
	return nil
}

// GetNodeIP 获取节点IP地址
func (nc *NodeCache) GetNodeIP(nodeName string) string {
	if nodeName == "" {
		return ""
	}
	if ip, ok := nc.Load(nodeName); ok {
		if ipStr, ok := ip.(string); ok {
			return ipStr
		}
	}
	return ""
}

// GetNodeIPOrUnknown 获取节点IP地址，如果不存在返回"Unknown"
func (nc *NodeCache) GetNodeIPOrUnknown(nodeName string) string {
	ip := nc.GetNodeIP(nodeName)
	if ip == "" {
		return "Unknown"
	}
	return ip
}
