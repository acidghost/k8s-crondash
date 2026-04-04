package k8s

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientSet(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		slog.Info("using in-cluster Kubernetes config")
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster client: %w", err)
		}
		return clientset, nil
	}

	slog.Info("in-cluster config unavailable, falling back to kubeconfig", "error", err)

	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig from %s: %w", kubeconfigPath, err)
	}

	slog.Info("using kubeconfig", "path", kubeconfigPath)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeconfig client: %w", err)
	}
	return clientset, nil
}
