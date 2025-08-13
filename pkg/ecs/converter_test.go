package ecs

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestConverter_Convert(t *testing.T) {
	tests := []struct {
		name      string
		podSpec   *corev1.PodSpec
		ecsConfig *ECSConfig
		options   ConversionOptions
		want      *ECSTaskDefinition
		wantErr   bool
	}{
		{
			name: "basic pod conversion",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:alpine",
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 80,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
			ecsConfig: &ECSConfig{
				Family: "test-family",
				CPU:    "256",
				Memory: "512",
			},
			options: ConversionOptions{},
			want: &ECSTaskDefinition{
				Family:                  "test-family",
				NetworkMode:             "awsvpc",
				RequiresCompatibilities: []string{"FARGATE"},
				CPU:                     "256",
				Memory:                  "512",
				ContainerDefinitions: []ECSContainerDefinition{
					{
						Name:              "nginx",
						Image:             "nginx:alpine",
						Essential:         true,
						CPU:               500,
						Memory:            512,
						MemoryReservation: 256,
						PortMappings: []ECSPortMapping{
							{
								ContainerPort: 80,
								Protocol:      "tcp",
							},
						},
						LogConfiguration: &ECSLogConfiguration{
							LogDriver: "awslogs",
							Options: map[string]string{
								"awslogs-group":  "/ecs/task",
								"awslogs-region": "us-east-1",
							},
						},
					},
				},
			},
		},
		{
			name: "pod with environment variables and secrets",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:latest",
						Env: []corev1.EnvVar{
							{
								Name:  "PLAIN_VAR",
								Value: "plain-value",
							},
							{
								Name: "SECRET_VAR",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "my-secret",
										},
										Key: "password",
									},
								},
							},
							{
								Name: "CONFIG_VAR",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "my-config",
										},
										Key: "config-key",
									},
								},
							},
						},
					},
				},
			},
			ecsConfig: &ECSConfig{
				Family: "test-env-family",
			},
			options: ConversionOptions{
				ParameterStorePrefix: "/myapp",
			},
			want: &ECSTaskDefinition{
				Family:                  "test-env-family",
				NetworkMode:             "awsvpc",
				RequiresCompatibilities: []string{"FARGATE"},
				ContainerDefinitions: []ECSContainerDefinition{
					{
						Name:      "app",
						Image:     "myapp:latest",
						Essential: true,
						Environment: []ECSKeyValuePair{
							{
								Name:  "PLAIN_VAR",
								Value: "plain-value",
							},
						},
						Secrets: []ECSSecret{
							{
								Name:      "SECRET_VAR",
								ValueFrom: "/myapp/test-namespace/secrets/my-secret/password",
							},
							{
								Name:      "CONFIG_VAR",
								ValueFrom: "/myapp/test-namespace/configmaps/my-config/config-key",
							},
						},
						LogConfiguration: &ECSLogConfiguration{
							LogDriver: "awslogs",
							Options: map[string]string{
								"awslogs-group":  "/ecs/task",
								"awslogs-region": "us-east-1",
							},
						},
					},
				},
			},
		},
		{
			name: "pod with init containers should fail",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:latest",
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:  "init",
						Image: "init:latest",
					},
				},
			},
			ecsConfig: &ECSConfig{
				Family: "test-init-family",
			},
			options: ConversionOptions{
				SkipUnsupportedFeatures: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter(tt.options)
			got, err := c.Convert(tt.podSpec, tt.ecsConfig, "test-namespace")

			if (err != nil) != tt.wantErr {
				t.Errorf("Converter.Convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Compare the results (simplified comparison)
			if got.Family != tt.want.Family {
				t.Errorf("Family = %v, want %v", got.Family, tt.want.Family)
			}

			if got.NetworkMode != tt.want.NetworkMode {
				t.Errorf("NetworkMode = %v, want %v", got.NetworkMode, tt.want.NetworkMode)
			}

			if len(got.ContainerDefinitions) != len(tt.want.ContainerDefinitions) {
				t.Errorf("ContainerDefinitions count = %v, want %v",
					len(got.ContainerDefinitions), len(tt.want.ContainerDefinitions))
				return
			}

			// Check first container definition
			if len(got.ContainerDefinitions) > 0 {
				gotContainer := got.ContainerDefinitions[0]
				wantContainer := tt.want.ContainerDefinitions[0]

				if gotContainer.Name != wantContainer.Name {
					t.Errorf("Container Name = %v, want %v", gotContainer.Name, wantContainer.Name)
				}

				if gotContainer.Image != wantContainer.Image {
					t.Errorf("Container Image = %v, want %v", gotContainer.Image, wantContainer.Image)
				}

				if gotContainer.Essential != wantContainer.Essential {
					t.Errorf("Container Essential = %v, want %v", gotContainer.Essential, wantContainer.Essential)
				}
			}
		})
	}
}

func TestConvertFromPod(t *testing.T) {
	converter := NewConverter(ConversionOptions{
		ParameterStorePrefix: "/test",
	})

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:alpine",
					Env: []corev1.EnvVar{
						{Name: "PLAIN", Value: "value"},
					},
				},
			},
		},
	}

	ecsConfig := &ECSConfig{
		Family: "test-pod",
	}

	taskDef, err := ConvertFromPod(converter, pod, ecsConfig)
	if err != nil {
		t.Fatalf("ConvertFromPod() error = %v", err)
	}

	if taskDef.Family != "test-pod" {
		t.Errorf("Family = %v, want test-pod", taskDef.Family)
	}

	if len(taskDef.ContainerDefinitions) != 1 {
		t.Errorf("ContainerDefinitions count = %v, want 1", len(taskDef.ContainerDefinitions))
	}
}
