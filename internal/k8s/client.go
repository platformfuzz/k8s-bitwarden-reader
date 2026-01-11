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

// NewK8sClient creates Kubernetes clients with in-cluster config or kubeconfig fallback
// Returns (nil, nil) if no Kubernetes config is found (standalone mode)
func NewK8sClient() (*K8sClients, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first (when running inside a Kubernetes cluster)
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		// Use default loading rules which handle KUBECONFIG env var and ~/.kube/config
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

		// Check if any kubeconfig file exists
		found := false
		if len(loadingRules.Precedence) > 0 {
			for _, path := range loadingRules.Precedence {
				if path != "" {
					if _, statErr := os.Stat(path); statErr == nil {
						found = true
						break
					}
				}
			}
		}

		if !found {
			// No Kubernetes config found - return nil for standalone mode
			return nil, nil
		}

		// Build config from kubeconfig
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
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
