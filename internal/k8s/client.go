package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClients holds both the standard and dynamic Kubernetes clients
type K8sClients struct {
	Clientset    kubernetes.Interface
	DynamicClient dynamic.Interface
}

// findKubeconfigFile checks if any kubeconfig file exists in the loading rules precedence
func findKubeconfigFile(loadingRules *clientcmd.ClientConfigLoadingRules) bool {
	if len(loadingRules.Precedence) == 0 {
		return false
	}
	for _, path := range loadingRules.Precedence {
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// buildKubeconfig builds a Kubernetes config from kubeconfig files
func buildKubeconfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if !findKubeconfigFile(loadingRules) {
		return nil, nil // No kubeconfig found
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	return config, nil
}

// NewK8sClient creates Kubernetes clients with in-cluster config or kubeconfig fallback
// Returns (nil, nil) if no Kubernetes config is found (standalone mode)
func NewK8sClient() (*K8sClients, error) {
	// Try in-cluster config first (when running inside a Kubernetes cluster)
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		config, err = buildKubeconfig()
		if err != nil {
			return nil, err
		}
		if config == nil {
			// No Kubernetes config found - return nil for standalone mode
			return nil, nil
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClients{
		Clientset:    clientset,
		DynamicClient: dynamicClient,
	}, nil
}
