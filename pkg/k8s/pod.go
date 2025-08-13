package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodService provides operations for working with Pods
type PodService struct {
	client *Client
}

// NewPodService creates a new PodService
func NewPodService(client *Client) *PodService {
	return &PodService{
		client: client,
	}
}

// ListPods lists all pods in the specified namespace
// If namespace is empty, lists pods in all namespaces
func (p *PodService) ListPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	return p.client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}

// GetPod gets a specific pod by name and namespace
func (p *PodService) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return p.client.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}
