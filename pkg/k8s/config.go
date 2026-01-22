package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	clientcmd_api "k8s.io/client-go/tools/clientcmd/api"

	"pod-monitor/internal/errors"
	"pod-monitor/internal/log"
)

func loadConfig(kubeconfig string) (*clientcmd_api.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
		log.Stdout.Debug("使用自定义 kubeconfig", "path", kubeconfig)
	} else {
		defaultPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
		if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
			log.Stdout.Warn("默认 kubeconfig 不存在", "path", defaultPath)
		}
		log.Stdout.Debug("使用默认 kubeconfig 路径", "path", defaultPath)
	}

	config, err := clientcmd.LoadFromFile(loadingRules.GetDefaultFilename())
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrConfigLoad,
			"loadConfig",
			err,
			"无法加载 kubeconfig 文件",
		)
	}

	return config, nil
}
