package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"pod-monitor/internal/errors"
	"pod-monitor/internal/log"
)

type Client struct {
	Clientset *kubernetes.Clientset
	Metrics   *metricsv.Clientset
	QPS       float32
	Burst     int
}

type Config struct {
	Kubeconfig string
	QPS        float32
	Burst      int
}

func NewClient(kubeconfig string, qps float32, burst int) (*Client, error) {
	log.Stdout.Info("创建 K8S 客户端",
		"kubeconfig", kubeconfig,
		"qps", qps,
		"burst", burst,
	)

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()

	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrConfigLoad,
			"NewClient",
			err,
			"创建 REST 配置失败",
		)
	}

	config.QPS = qps
	config.Burst = burst

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrK8SClient,
			"NewClient/create_clientset",
			err,
			"创建 Kubernetes 客户端失败",
		)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrMetricsClient,
			"NewClient/create_metrics",
			err,
			"创建 Metrics 客户端失败",
		)
	}

	log.Stdout.Info("K8S 客户端创建成功")

	return &Client{
		Clientset: clientset,
		Metrics:   metricsClient,
		QPS:       qps,
		Burst:     burst,
	}, nil
}
