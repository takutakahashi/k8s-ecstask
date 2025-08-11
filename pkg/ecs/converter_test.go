package ecs

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestConverter_Convert(t *testing.T) {
	tests := []struct {
		name    string
		spec    *XPodSpec
		options ConversionOptions
		want    *ECSTaskDefinition
		wantErr bool
	}{
		{
			name: "basic pod conversion",
			spec: &XPodSpec{
				Family: "test-family",
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
			spec: &XPodSpec{
				Family: "test-env-family",
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
								ValueFrom: "/myapp/secrets/my-secret/password",
							},
							{
								Name:      "CONFIG_VAR",
								ValueFrom: "/myapp/configmaps/my-config/config-key",
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
			name: "pod with volumes and mounts",
			spec: &XPodSpec{
				Family: "test-volumes-family",
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:latest",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data-volume",
								MountPath: "/data",
								ReadOnly:  false,
							},
							{
								Name:      "config-volume",
								MountPath: "/etc/config",
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "data-volume",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/host/data",
							},
						},
					},
					{
						Name: "config-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			options: ConversionOptions{},
			want: &ECSTaskDefinition{
				Family:                  "test-volumes-family",
				NetworkMode:             "awsvpc",
				RequiresCompatibilities: []string{"FARGATE"},
				ContainerDefinitions: []ECSContainerDefinition{
					{
						Name:      "app",
						Image:     "myapp:latest",
						Essential: true,
						MountPoints: []ECSMountPoint{
							{
								SourceVolume:  "data-volume",
								ContainerPath: "/data",
								ReadOnly:      false,
							},
							{
								SourceVolume:  "config-volume",
								ContainerPath: "/etc/config",
								ReadOnly:      true,
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
				Volumes: []ECSVolume{
					{
						Name: "data-volume",
						Host: &ECSHostVolume{
							SourcePath: "/host/data",
						},
					},
					{
						Name: "config-volume",
						Host: &ECSHostVolume{},
					},
				},
			},
		},
		{
			name: "pod with custom task and execution roles",
			spec: &XPodSpec{
				Family:           "test-roles-family",
				TaskRoleArn:      "arn:aws:iam::123456789012:role/TaskRole",
				ExecutionRoleArn: "arn:aws:iam::123456789012:role/ExecutionRole",
				NetworkMode:      "bridge",
				RequiresCompatibilities: []string{"EC2"},
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:latest",
					},
				},
				Tags: map[string]string{
					"Environment": "test",
					"Project":     "myproject",
				},
			},
			options: ConversionOptions{},
			want: &ECSTaskDefinition{
				Family:                  "test-roles-family",
				TaskRoleArn:             "arn:aws:iam::123456789012:role/TaskRole",
				ExecutionRoleArn:        "arn:aws:iam::123456789012:role/ExecutionRole",
				NetworkMode:             "bridge",
				RequiresCompatibilities: []string{"EC2"},
				ContainerDefinitions: []ECSContainerDefinition{
					{
						Name:      "app",
						Image:     "myapp:latest",
						Essential: true,
						LogConfiguration: &ECSLogConfiguration{
							LogDriver: "awslogs",
							Options: map[string]string{
								"awslogs-group":  "/ecs/task",
								"awslogs-region": "us-east-1",
							},
						},
					},
				},
				Tags: []ECSTag{
					{Key: "Environment", Value: "test"},
					{Key: "Project", Value: "myproject"},
				},
			},
		},
		{
			name: "pod with init containers should fail",
			spec: &XPodSpec{
				Family: "test-init-family",
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
			options: ConversionOptions{
				SkipUnsupportedFeatures: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter(tt.options)
			got, err := c.Convert(tt.spec)
			
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
				t.Errorf("ContainerDefinitions count = %v, want %v", len(got.ContainerDefinitions), len(tt.want.ContainerDefinitions))
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

func TestConverter_ConvertEnvironment(t *testing.T) {
	converter := NewConverter(ConversionOptions{
		ParameterStorePrefix: "/test",
	})

	container := corev1.Container{
		Name: "test",
		Env: []corev1.EnvVar{
			{Name: "PLAIN", Value: "value"},
			{
				Name: "SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "mysecret"},
						Key:                  "mykey",
					},
				},
			},
			{
				Name: "CONFIG",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "myconfig"},
						Key:                  "configkey",
					},
				},
			},
		},
	}

	containerDef := &ECSContainerDefinition{}
	err := converter.convertEnvironment(container, containerDef)
	
	if err != nil {
		t.Fatalf("convertEnvironment() error = %v", err)
	}

	// Check environment variables
	if len(containerDef.Environment) != 1 {
		t.Errorf("Environment count = %v, want 1", len(containerDef.Environment))
	}
	
	if containerDef.Environment[0].Name != "PLAIN" || containerDef.Environment[0].Value != "value" {
		t.Errorf("Environment[0] = %+v, want {Name: PLAIN, Value: value}", containerDef.Environment[0])
	}

	// Check secrets
	if len(containerDef.Secrets) != 2 {
		t.Errorf("Secrets count = %v, want 2", len(containerDef.Secrets))
	}

	expectedSecrets := map[string]string{
		"SECRET": "/test/secrets/mysecret/mykey",
		"CONFIG": "/test/configmaps/myconfig/configkey",
	}

	for _, secret := range containerDef.Secrets {
		expectedPath, exists := expectedSecrets[secret.Name]
		if !exists {
			t.Errorf("Unexpected secret: %s", secret.Name)
			continue
		}
		
		if secret.ValueFrom != expectedPath {
			t.Errorf("Secret %s ValueFrom = %s, want %s", secret.Name, secret.ValueFrom, expectedPath)
		}
	}
}

func TestConverter_ConvertResources(t *testing.T) {
	converter := NewConverter(ConversionOptions{})

	container := corev1.Container{
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	}

	containerDef := &ECSContainerDefinition{}
	err := converter.convertResources(container, containerDef)
	
	if err != nil {
		t.Fatalf("convertResources() error = %v", err)
	}

	if containerDef.CPU != 1000 {
		t.Errorf("CPU = %v, want 1000", containerDef.CPU)
	}

	if containerDef.Memory != 1024 {
		t.Errorf("Memory = %v, want 1024", containerDef.Memory)
	}

	if containerDef.MemoryReservation != 512 {
		t.Errorf("MemoryReservation = %v, want 512", containerDef.MemoryReservation)
	}
}