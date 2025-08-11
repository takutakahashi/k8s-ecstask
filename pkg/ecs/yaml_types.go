package ecs

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodWithECSConfig represents a Kubernetes Pod with additional ECS configuration
type PodWithECSConfig struct {
	// Standard Kubernetes Pod fields
	metav1.TypeMeta   `yaml:",inline"`
	metav1.ObjectMeta `yaml:"metadata,omitempty"`
	Spec              corev1.PodSpec   `yaml:"spec,omitempty"`
	Status            corev1.PodStatus `yaml:"status,omitempty"`
	
	// ECS-specific configuration
	ECSConfig ECSConfig `yaml:"ecsConfig,omitempty"`
}

// ConvertFromPod converts a standard Kubernetes Pod and ECSConfig to ECS Task Definition
func ConvertFromPod(converter *Converter, pod *corev1.Pod, ecsConfig *ECSConfig) (*ECSTaskDefinition, error) {
	return converter.Convert(&pod.Spec, ecsConfig, pod.Namespace)
}

// ConvertFromPodWithConfig converts a PodWithECSConfig to ECS Task Definition
func ConvertFromPodWithConfig(converter *Converter, podWithConfig *PodWithECSConfig) (*ECSTaskDefinition, error) {
	return converter.Convert(&podWithConfig.Spec, &podWithConfig.ECSConfig, podWithConfig.Namespace)
}