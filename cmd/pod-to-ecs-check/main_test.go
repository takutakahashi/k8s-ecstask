package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestValidatePod(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		skipWarnings bool
		wantConvert  bool
		wantErrors   int
		wantWarnings int
	}{
		{
			name: "Simple convertible pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  true,
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name: "Pod with init containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-init",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init",
							Image: "busybox",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  false,
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name: "Pod with host network",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-hostnet",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  false,
			wantErrors:   1,
			wantWarnings: 0,
		},
		{
			name: "Pod with privileged container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-privileged",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: &[]bool{true}[0],
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  false,
			wantErrors:   1,
			wantWarnings: 0,
		},
		{
			name: "Pod with secret references",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-secrets",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							Env: []corev1.EnvVar{
								{
									Name: "SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "app-secret",
											},
											Key: "key",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  true,
			wantErrors:   0,
			wantWarnings: 1,
		},
		{
			name: "Pod with PVC",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-pvc",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "data-pvc",
								},
							},
						},
					},
				},
			},
			skipWarnings: false,
			wantConvert:  false,
			wantErrors:   1,
			wantWarnings: 0,
		},
		{
			name: "Pod with warnings but skip warnings enabled",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-skip-warnings",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "custom-sa",
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			skipWarnings: true,
			wantConvert:  true,
			wantErrors:   0,
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePod(tt.pod, tt.skipWarnings)

			if result.CanConvert != tt.wantConvert {
				t.Errorf("validatePod() CanConvert = %v, want %v", result.CanConvert, tt.wantConvert)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("validatePod() got %d errors, want %d", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %s", err)
				}
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("validatePod() got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %s", warn)
				}
			}
		})
	}
}

func TestValidateContainer(t *testing.T) {
	tests := []struct {
		name         string
		container    corev1.Container
		wantErrors   int
		wantWarnings int
	}{
		{
			name: "Container with no image",
			container: corev1.Container{
				Name: "app",
			},
			wantErrors:   1,
			wantWarnings: 1,
		},
		{
			name: "Container with field ref env var",
			container: corev1.Container{
				Name:  "app",
				Image: "app:latest",
				Env: []corev1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name: "Container with probes",
			container: corev1.Container{
				Name:  "app",
				Image: "app:latest",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(8080),
						},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/ready",
							Port: intstr.FromInt(8080),
						},
					},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			wantErrors:   0,
			wantWarnings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Errors:   []string{},
				Warnings: []string{},
			}

			validateContainer(&tt.container, result)

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("validateContainer() got %d errors, want %d", len(result.Errors), tt.wantErrors)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("validateContainer() got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestValidateVolume(t *testing.T) {
	tests := []struct {
		name       string
		volume     corev1.Volume
		wantErrors int
	}{
		{
			name: "EmptyDir volume",
			volume: corev1.Volume{
				Name: "temp",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			wantErrors: 0,
		},
		{
			name: "PVC volume",
			volume: corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "data-pvc",
					},
				},
			},
			wantErrors: 1,
		},
		{
			name: "NFS volume",
			volume: corev1.Volume{
				Name: "nfs",
				VolumeSource: corev1.VolumeSource{
					NFS: &corev1.NFSVolumeSource{
						Server: "nfs.example.com",
						Path:   "/data",
					},
				},
			},
			wantErrors: 1,
		},
		{
			name: "HostPath volume",
			volume: corev1.Volume{
				Name: "host",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/log",
					},
				},
			},
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Errors:   []string{},
				Warnings: []string{},
			}

			validateVolume(&tt.volume, result)

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("validateVolume() got %d errors, want %d", len(result.Errors), tt.wantErrors)
			}
		})
	}
}
