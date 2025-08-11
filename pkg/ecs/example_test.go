package ecs_test

import (
	"encoding/json"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/takutakahashi/k8s-ecstask/pkg/ecs"
)

func ExampleConverter_Convert() {
	// Create a new converter with options
	options := ecs.ConversionOptions{
		ParameterStorePrefix:    "/myapp",
		DefaultLogDriver:        "awslogs",
		DefaultExecutionRoleArn: "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
		DefaultTaskRoleArn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
		DefaultLogOptions: map[string]string{
			"awslogs-group":         "/ecs/myapp",
			"awslogs-region":        "ap-northeast-1",
			"awslogs-stream-prefix": "ecs",
		},
		SkipUnsupportedFeatures: true,
	}

	converter := ecs.NewConverter(options)

	// Define a standard Kubernetes Pod
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-app-pod",
			Labels: map[string]string{
				"app":         "web-app",
				"environment": "production",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.20-alpine",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("256m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "ENV",
							Value: "production",
						},
						{
							Name: "DB_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "db-credentials",
									},
									Key: "password",
								},
							},
						},
						{
							Name: "API_CONFIG",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "api-config",
									},
									Key: "config.yaml",
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "static-content",
							MountPath: "/usr/share/nginx/html",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "static-content",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/opt/static-content",
						},
					},
				},
			},
		},
	}

	// ECS-specific configuration
	ecsConfig := &ecs.ECSConfig{
		Family:                  "web-app",
		CPU:                     "256",
		Memory:                  "512",
		NetworkMode:             "awsvpc",
		RequiresCompatibilities: []string{"FARGATE"},
		Tags: map[string]string{
			"Environment": "production",
			"Team":        "platform",
			"Project":     "web-app",
		},
	}

	// Convert to ECS task definition
	taskDef, err := ecs.ConvertFromPod(converter, pod, ecsConfig)
	if err != nil {
		log.Fatalf("Failed to convert: %v", err)
	}

	// Marshal to JSON for output
	jsonBytes, err := json.MarshalIndent(taskDef, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func ExampleConverter_Convert_minimal() {
	// Minimal example with default settings
	converter := ecs.NewConverter(ecs.ConversionOptions{})

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "hello-world:latest",
				},
			},
		},
	}

	ecsConfig := &ecs.ECSConfig{
		Family: "simple-app",
	}

	taskDef, err := ecs.ConvertFromPod(converter, pod, ecsConfig)
	if err != nil {
		log.Fatalf("Failed to convert: %v", err)
	}

	fmt.Printf("Family: %s\n", taskDef.Family)
	fmt.Printf("NetworkMode: %s\n", taskDef.NetworkMode)
	fmt.Printf("Container Count: %d\n", len(taskDef.ContainerDefinitions))
	fmt.Printf("Container Name: %s\n", taskDef.ContainerDefinitions[0].Name)
	
	// Output:
	// Family: simple-app
	// NetworkMode: awsvpc
	// Container Count: 1
	// Container Name: app
}