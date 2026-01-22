//go:build integration
// +build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_PodCommand tests the pod command help
func TestCLI_PodCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	cmd := exec.Command("go", "run", "./cmd/pod-monitor", "pod", "--help")
	cmd.Dir = getProjectRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pod --help failed: %v\nOutput: %s", err, output)
	}

	if len(output) == 0 {
		t.Error("Expected help output, got empty")
	}

	// Verify some expected content in help output
	outputStr := string(output)
	if !contains(outputStr, "检查 Kubernetes Pod 的重启情况和异常状态") {
		t.Error("Expected pod help description in output")
	}
}

// TestCLI_NodeCommand tests the node command help
func TestCLI_NodeCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	cmd := exec.Command("go", "run", "./cmd/pod-monitor", "node", "--help")
	cmd.Dir = getProjectRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node --help failed: %v\nOutput: %s", err, output)
	}

	if len(output) == 0 {
		t.Error("Expected help output, got empty")
	}

	// Verify some expected content in help output
	outputStr := string(output)
	if !contains(outputStr, "显示 Kubernetes 节点的资源使用情况") {
		t.Error("Expected node help description in output")
	}
}

// TestCLI_RootCommand tests the root command help
func TestCLI_RootCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	cmd := exec.Command("go", "run", "./cmd/pod-monitor", "--help")
	cmd.Dir = getProjectRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("root --help failed: %v\nOutput: %s", err, output)
	}

	if len(output) == 0 {
		t.Error("Expected help output, got empty")
	}

	// Verify some expected content in help output
	outputStr := string(output)
	if !contains(outputStr, "Pod Monitor") {
		t.Error("Expected 'Pod Monitor' in help output")
	}
}

// TestCLI_Build tests that the application builds successfully
func TestCLI_Build(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	cmd := exec.Command("go", "build", "-o", "pod-monitor-test.exe", "./cmd/pod-monitor")
	cmd.Dir = getProjectRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\nOutput: %s", err, output)
	}

	// Clean up the test binary
	testBinary := filepath.Join(getProjectRoot(), "pod-monitor-test.exe")
	os.Remove(testBinary)
}

// getProjectRoot returns the absolute path to the project root directory
func getProjectRoot() string {
	dir, _ := os.Getwd()
	// Check if go.mod exists in current directory
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return dir
	}
	// Go up two levels from test/integration
	parent := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(parent, "go.mod")); err == nil {
		return parent
	}
	grandParent := filepath.Dir(parent)
	if _, err := os.Stat(filepath.Join(grandParent, "go.mod")); err == nil {
		return grandParent
	}
	// Fallback - return current directory
	return dir
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
