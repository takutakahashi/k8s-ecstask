package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// ValidationResult represents the result of validating a single Pod
type ValidationResult struct {
	PodName         string   `json:"podName"`
	Namespace       string   `json:"namespace"`
	CanConvert      bool     `json:"canConvert"`
	Errors          []string `json:"errors,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	UnsupportedInfo []string `json:"unsupportedInfo,omitempty"`
}

// ValidationSummary represents the overall validation summary
type ValidationSummary struct {
	TotalPods       int                `json:"totalPods"`
	ConvertiblePods int                `json:"convertiblePods"`
	FailedPods      int                `json:"failedPods"`
	Results         []ValidationResult `json:"results"`
}

func main() {
	var (
		namespace    = flag.String("namespace", "", "Kubernetes namespace to check (default: all namespaces)")
		kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
		outputFormat = flag.String("output", "text", "Output format: text or json")
		skipWarnings = flag.Bool("skip-warnings", false, "Skip validation warnings")
	)
	flag.Parse()

	// Setup kubeconfig
	if *kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			*kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// Create Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Get pods
	ctx := context.Background()
	var pods *corev1.PodList

	if *namespace != "" {
		pods, err = clientset.CoreV1().Pods(*namespace).List(ctx, metav1.ListOptions{})
	} else {
		pods, err = clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		log.Fatalf("Failed to list pods: %v", err)
	}

	// Validate pods
	summary := ValidationSummary{
		TotalPods: len(pods.Items),
		Results:   make([]ValidationResult, 0, len(pods.Items)),
	}

	for _, pod := range pods.Items {
		result := validatePod(&pod, *skipWarnings)
		summary.Results = append(summary.Results, result)

		if result.CanConvert {
			summary.ConvertiblePods++
		} else {
			summary.FailedPods++
		}
	}

	// Output results
	if *outputFormat == "json" {
		outputJSON(summary)
	} else {
		outputText(summary)
	}

	// Exit with error if any pods failed
	if summary.FailedPods > 0 {
		os.Exit(1)
	}
}

func validatePod(pod *corev1.Pod, skipWarnings bool) ValidationResult {
	result := ValidationResult{
		PodName:         pod.Name,
		Namespace:       pod.Namespace,
		CanConvert:      true,
		Errors:          []string{},
		Warnings:        []string{},
		UnsupportedInfo: []string{},
	}

	// Check for init containers
	if len(pod.Spec.InitContainers) > 0 {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			fmt.Sprintf("Pod has %d init containers which are not directly supported in ECS", len(pod.Spec.InitContainers)))
		result.CanConvert = false
	}

	// Validate containers
	for _, container := range pod.Spec.Containers {
		validateContainer(&container, &result)
	}

	// Validate volumes
	for _, volume := range pod.Spec.Volumes {
		validateVolume(&volume, &result)
	}

	// Check for unsupported pod features
	if pod.Spec.HostNetwork {
		result.Errors = append(result.Errors, "Pod uses host network mode which may require special ECS configuration")
		result.CanConvert = false
	}

	if pod.Spec.HostPID {
		result.Errors = append(result.Errors, "Pod uses host PID namespace which is not supported in ECS Fargate")
		result.CanConvert = false
	}

	if pod.Spec.HostIPC {
		result.Errors = append(result.Errors, "Pod uses host IPC namespace which is not supported in ECS Fargate")
		result.CanConvert = false
	}

	// Check service account
	if pod.Spec.ServiceAccountName != "" && pod.Spec.ServiceAccountName != "default" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Pod uses service account '%s' - ensure appropriate IAM roles are configured for ECS", pod.Spec.ServiceAccountName))
	}

	// Security context warnings
	if pod.Spec.SecurityContext != nil {
		if pod.Spec.SecurityContext.RunAsUser != nil {
			result.Warnings = append(result.Warnings, "Pod specifies RunAsUser - ECS uses task role for permissions")
		}
		if pod.Spec.SecurityContext.FSGroup != nil {
			result.Warnings = append(result.Warnings, "Pod specifies FSGroup - this is not directly supported in ECS")
		}
	}

	// Apply skip warnings if requested
	if skipWarnings && result.CanConvert {
		result.Warnings = []string{}
	}

	return result
}

func validateContainer(container *corev1.Container, result *ValidationResult) {
	// Check for required fields
	if container.Image == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("Container '%s' has no image specified", container.Name))
		result.CanConvert = false
	}

	// Check environment variables
	for _, env := range container.Env {
		if env.ValueFrom != nil {
			if env.ValueFrom.FieldRef != nil {
				result.UnsupportedInfo = append(result.UnsupportedInfo,
					fmt.Sprintf("Container '%s' uses field reference for env var '%s' which is not supported", container.Name, env.Name))
			}
			if env.ValueFrom.ResourceFieldRef != nil {
				result.UnsupportedInfo = append(result.UnsupportedInfo,
					fmt.Sprintf("Container '%s' uses resource field reference for env var '%s' which is not supported", container.Name, env.Name))
			}
			if env.ValueFrom.SecretKeyRef != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Container '%s' references secret '%s' - ensure it's migrated to Parameter Store", container.Name, env.ValueFrom.SecretKeyRef.Name))
			}
			if env.ValueFrom.ConfigMapKeyRef != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Container '%s' references configmap '%s' - ensure it's migrated to Parameter Store", container.Name, env.ValueFrom.ConfigMapKeyRef.Name))
			}
		}
	}

	// Check probes
	if container.LivenessProbe != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Container '%s' has liveness probe - ECS health checks work differently", container.Name))
	}
	if container.ReadinessProbe != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Container '%s' has readiness probe - ECS uses target group health checks", container.Name))
	}

	// Check security context
	if container.SecurityContext != nil {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Container '%s' requires privileged mode which is not supported in ECS Fargate", container.Name))
			result.CanConvert = false
		}
		if container.SecurityContext.Capabilities != nil {
			if len(container.SecurityContext.Capabilities.Add) > 0 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Container '%s' adds Linux capabilities - verify ECS task role permissions", container.Name))
			}
		}
	}

	// Check resource limits
	if container.Resources.Limits == nil && container.Resources.Requests == nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Container '%s' has no resource limits/requests - ECS requires CPU and memory specification", container.Name))
	}
}

func validateVolume(volume *corev1.Volume, result *ValidationResult) {
	if volume.PersistentVolumeClaim != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Volume '%s' uses PersistentVolumeClaim which is not supported in ECS", volume.Name))
		result.CanConvert = false
	}

	if volume.NFS != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Volume '%s' uses NFS which requires EFS configuration in ECS", volume.Name))
		result.CanConvert = false
	}

	if volume.Secret != nil {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			fmt.Sprintf("Volume '%s' mounts secret '%s' - use Parameter Store or Secrets Manager instead", volume.Name, volume.Secret.SecretName))
	}

	if volume.ConfigMap != nil {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			fmt.Sprintf("Volume '%s' mounts configmap '%s' - use Parameter Store instead", volume.Name, volume.ConfigMap.Name))
	}

	if volume.HostPath != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Volume '%s' uses hostPath - ensure the path is available in ECS", volume.Name))
	}
}

func outputJSON(summary ValidationSummary) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		log.Fatalf("Failed to encode JSON: %v", err)
	}
}

func outputText(summary ValidationSummary) {
	fmt.Printf("Pod to ECS Convertibility Check Results\n")
	fmt.Printf("========================================\n\n")
	fmt.Printf("Total Pods: %d\n", summary.TotalPods)
	fmt.Printf("Convertible: %d\n", summary.ConvertiblePods)
	fmt.Printf("Failed: %d\n\n", summary.FailedPods)

	for _, result := range summary.Results {
		status := "✓ CONVERTIBLE"
		if !result.CanConvert {
			status = "✗ NOT CONVERTIBLE"
		}

		fmt.Printf("Pod: %s/%s - %s\n", result.Namespace, result.PodName, status)

		if len(result.Errors) > 0 {
			fmt.Printf("  Errors:\n")
			for _, err := range result.Errors {
				fmt.Printf("    - %s\n", err)
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Printf("  Warnings:\n")
			for _, warn := range result.Warnings {
				fmt.Printf("    - %s\n", warn)
			}
		}

		if len(result.UnsupportedInfo) > 0 {
			fmt.Printf("  Unsupported Features:\n")
			for _, info := range result.UnsupportedInfo {
				fmt.Printf("    - %s\n", info)
			}
		}

		fmt.Println()
	}

	// Summary recommendations
	if summary.FailedPods > 0 {
		fmt.Printf("Summary: %d pods cannot be converted to ECS tasks without modifications.\n", summary.FailedPods)
		fmt.Printf("Please review the errors above and modify the pod specifications accordingly.\n")
	} else if summary.ConvertiblePods == summary.TotalPods {
		fmt.Printf("Summary: All pods can be converted to ECS tasks!\n")
		warnings := 0
		for _, result := range summary.Results {
			warnings += len(result.Warnings)
		}
		if warnings > 0 {
			fmt.Printf("However, there are %d warnings that should be reviewed.\n", warnings)
		}
	}
}
