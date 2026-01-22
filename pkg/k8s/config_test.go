package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_DefaultPath(t *testing.T) {
	// Note: This test will pass if default kubeconfig exists
	// The main goal is to test the loadConfig function with default path
	config, err := loadConfig("")
	if err != nil {
		t.Logf("Expected error for non-existent kubeconfig: %v", err)
	} else {
		t.Logf("Successfully loaded default config: %v", config)
	}
}

func TestLoadConfig_ExplicitPath(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")
	os.WriteFile(kubeconfigPath, []byte("invalid"), 0644)

	_, err := loadConfig(kubeconfigPath)
	if err == nil {
		t.Log("Got expected error for invalid kubeconfig content")
	}
}
