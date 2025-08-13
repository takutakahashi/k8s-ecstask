package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidatePod(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		skipWarnings bool
		wantConvert  bool
		minErrors    int
		minWarnings  int
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
			minErrors:    0,
			minWarnings:  0,
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
			wantConvert:  true, // SkipUnsupportedFeatures=true なので変換可能
			minErrors:    0,
			minWarnings:  0,
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
			wantConvert:  true,
			minErrors:    0,
			minWarnings:  1,
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
			minErrors:    0,
			minWarnings:  1,
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
			minErrors:    0,
			minWarnings:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePod(tt.pod, tt.skipWarnings)

			if result.CanConvert != tt.wantConvert {
				t.Errorf("validatePod() CanConvert = %v, want %v", result.CanConvert, tt.wantConvert)
			}

			if len(result.Errors) < tt.minErrors {
				t.Errorf("validatePod() got %d errors, want at least %d",
					len(result.Errors), tt.minErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %s", err)
				}
			}

			if len(result.Warnings) < tt.minWarnings {
				t.Errorf("validatePod() got %d warnings, want at least %d",
					len(result.Warnings), tt.minWarnings)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %s", warn)
				}
			}
		})
	}
}

func TestCategorizeConversionError(t *testing.T) {
	tests := []struct {
		name            string
		errorMsg        string
		wantErrors      int
		wantUnsupported int
	}{
		{
			name:            "Init containers error",
			errorMsg:        "init containers are not supported",
			wantErrors:      0,
			wantUnsupported: 1,
		},
		{
			name:            "Volume error",
			errorMsg:        "secret/configmap volumes are not supported",
			wantErrors:      0,
			wantUnsupported: 1,
		},
		{
			name:            "Generic error",
			errorMsg:        "some other conversion error",
			wantErrors:      1,
			wantUnsupported: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Errors:          []string{},
				UnsupportedInfo: []string{},
			}

			err := &mockError{msg: tt.errorMsg}
			categorizeConversionError(err, result)

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("categorizeConversionError() got %d errors, want %d",
					len(result.Errors), tt.wantErrors)
			}

			if len(result.UnsupportedInfo) != tt.wantUnsupported {
				t.Errorf("categorizeConversionError() got %d unsupported, want %d",
					len(result.UnsupportedInfo), tt.wantUnsupported)
			}
		})
	}
}

// mockError implements error interface for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestAddWarnings(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		wantWarnings int
	}{
		{
			name: "Pod with no warnings",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple-pod",
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
				},
			},
			wantWarnings: 0,
		},
		{
			name: "Pod with host network",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host-net-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
						},
					},
				},
			},
			wantWarnings: 2, // host network + no resources
		},
		{
			name: "Pod with service account",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "custom-sa",
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "app:latest",
						},
					},
				},
			},
			wantWarnings: 2, // service account + no resources
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Warnings: []string{},
			}

			addWarnings(tt.pod, result)

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("addWarnings() got %d warnings, want %d",
					len(result.Warnings), tt.wantWarnings)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %s", warn)
				}
			}
		})
	}
}
