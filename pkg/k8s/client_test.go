package k8s

import (
	"os"
	"testing"
)

func TestClient_QPS_Burst(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 K8S 连接的测试")
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("未设置 KUBECONFIG 环境变量")
	}

	client, err := NewClient(kubeconfig, 50, 100)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.Clientset == nil {
		t.Error("Clientset is nil")
	}

	if client.Metrics == nil {
		t.Error("Metrics client is nil")
	}
}
