package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// ClientConfig holds configuration for creating Kubernetes clients
type ClientConfig struct {
	KubeconfigPath string
}

// Client wraps the Kubernetes clientset with additional functionality
type Client struct {
	Clientset kubernetes.Interface
	config    *rest.Config
}

// NewClient creates a new Kubernetes client with the given configuration
func NewClient(cfg ClientConfig) (*Client, error) {
	config, err := buildConfig(cfg.KubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset: clientset,
		config:    config,
	}, nil
}

// NewClientFromConfig creates a client from an existing rest.Config
func NewClientFromConfig(config *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset: clientset,
		config:    config,
	}, nil
}

// Config returns the underlying rest.Config
func (c *Client) Config() *rest.Config {
	return c.config
}

// buildConfig creates a rest.Config from kubeconfig path
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	// If no kubeconfig path provided, try to find default location
	if kubeconfigPath == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// Check if kubeconfig file exists
	if kubeconfigPath != "" {
		if _, err := os.Stat(kubeconfigPath); err == nil {
			// Use kubeconfig file
			return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		}
	}

	// Fall back to in-cluster config
	return rest.InClusterConfig()
}
