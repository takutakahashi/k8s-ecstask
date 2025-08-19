package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodService_ListPods(t *testing.T) {
	// Create a fake clientset with some test pods
	fakeClientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-1",
				Namespace: "default",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-2",
				Namespace: "kube-system",
			},
		},
	)

	client := &Client{
		Clientset: fakeClientset,
	}

	podService := NewPodService(client)
	ctx := context.Background()

	// Test listing all pods
	pods, err := podService.ListPods(ctx, "")
	if err != nil {
		t.Fatalf("ListPods() failed: %v", err)
	}

	if len(pods.Items) != 2 {
		t.Errorf("ListPods() got %d pods, want 2", len(pods.Items))
	}

	// Test listing pods in specific namespace
	pods, err = podService.ListPods(ctx, "default")
	if err != nil {
		t.Fatalf("ListPods() failed: %v", err)
	}

	if len(pods.Items) != 1 {
		t.Errorf("ListPods() got %d pods in default namespace, want 1", len(pods.Items))
	}

	if pods.Items[0].Name != "test-pod-1" {
		t.Errorf("ListPods() got pod name %s, want test-pod-1", pods.Items[0].Name)
	}
}

func TestPodService_GetPod(t *testing.T) {
	// Create a fake clientset with a test pod
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
				},
			},
		},
	}

	fakeClientset := fake.NewSimpleClientset(testPod)

	client := &Client{
		Clientset: fakeClientset,
	}

	podService := NewPodService(client)
	ctx := context.Background()

	// Test getting existing pod
	pod, err := podService.GetPod(ctx, "default", "test-pod")
	if err != nil {
		t.Fatalf("GetPod() failed: %v", err)
	}

	if pod.Name != "test-pod" {
		t.Errorf("GetPod() got pod name %s, want test-pod", pod.Name)
	}

	if len(pod.Spec.Containers) != 1 {
		t.Errorf("GetPod() got %d containers, want 1", len(pod.Spec.Containers))
	}

	// Test getting non-existent pod
	_, err = podService.GetPod(ctx, "default", "non-existent")
	if err == nil {
		t.Error("GetPod() should have failed for non-existent pod")
	}
}
