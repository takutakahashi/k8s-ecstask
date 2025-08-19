package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/takutakahashi/k8s-ecstask/pkg/ecs"
	"github.com/takutakahashi/k8s-ecstask/pkg/k8s"
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
		namespace = flag.String("namespace", "",
			"Kubernetes namespace to check (default: all namespaces)")
		kubeconfig = flag.String("kubeconfig", "",
			"Path to kubeconfig file (default: ~/.kube/config)")
		outputFormat = flag.String("output", "text", "Output format: text or json")
		skipWarnings = flag.Bool("skip-warnings", false, "Skip validation warnings")
	)
	flag.Parse()

	// Create Kubernetes client
	client, err := k8s.NewClient(k8s.ClientConfig{
		KubeconfigPath: *kubeconfig,
	})
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}

	// Create pod service
	podService := k8s.NewPodService(client)

	// Get pods
	ctx := context.Background()
	pods, err := podService.ListPods(ctx, *namespace)
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

	// Create converter with default options
	options := ecs.ConversionOptions{
		ParameterStorePrefix: "/pods",
		DefaultLogDriver:     "awslogs",
		DefaultLogOptions: map[string]string{
			"awslogs-group":  "/ecs/pods",
			"awslogs-region": "us-east-1",
		},
		SkipUnsupportedFeatures: true,
		DefaultExecutionRoleArn: "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
		DefaultTaskRoleArn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
	}

	converter := ecs.NewConverter(options)

	// Create minimal ECS config for validation
	ecsConfig := &ecs.ECSConfig{
		Family:                  fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		RequiresCompatibilities: []string{"FARGATE"},
		CPU:                     "256",
		Memory:                  "512",
	}

	// Try to convert using the existing converter
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	_, err := converter.Convert(&pod.Spec, ecsConfig, namespace)
	if err != nil {
		// Parse the error to categorize it
		result.CanConvert = false
		categorizeConversionError(err, &result)
	}

	// Additional validation checks for warnings
	addWarnings(pod, &result)

	// Apply skip warnings if requested
	if skipWarnings && result.CanConvert {
		result.Warnings = []string{}
	}

	return result
}

func categorizeConversionError(err error, result *ValidationResult) {
	errStr := err.Error()

	// Check for specific error patterns
	if strings.Contains(errStr, "init containers") {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			"Pod has init containers which are not directly supported in ECS")
	}

	if strings.Contains(errStr, "secret/configmap volumes") {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			"Pod uses secret/configmap volumes - use Parameter Store instead")
	}

	if strings.Contains(errStr, "field references") {
		result.UnsupportedInfo = append(result.UnsupportedInfo,
			"Pod uses field references which are not supported in ECS")
	}

	if strings.Contains(errStr, "unsupported volume type") {
		result.Errors = append(result.Errors,
			"Pod uses unsupported volume types for ECS")
	}

	// Generic error if no specific pattern matched
	if len(result.Errors) == 0 && len(result.UnsupportedInfo) == 0 {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Conversion failed: %s", errStr))
	}
}

func addWarnings(pod *corev1.Pod, result *ValidationResult) {
	// Check for potential issues that don't prevent conversion but should be noted

	// Check for host networking
	if pod.Spec.HostNetwork {
		result.Warnings = append(result.Warnings,
			"Pod uses host network - ensure ECS task configuration supports this")
	}

	// Check for privileged containers
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil &&
			container.SecurityContext.Privileged != nil &&
			*container.SecurityContext.Privileged {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Container '%s' requires privileged mode - "+
					"not supported in ECS Fargate", container.Name))
		}

		// Check for missing resource specifications
		if container.Resources.Limits == nil && container.Resources.Requests == nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Container '%s' has no resource limits/requests - "+
					"ECS requires CPU and memory specification", container.Name))
		}

		// Check for probes
		if container.LivenessProbe != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Container '%s' has liveness probe - "+
					"ECS health checks work differently", container.Name))
		}
		if container.ReadinessProbe != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Container '%s' has readiness probe - "+
					"ECS uses target group health checks", container.Name))
		}

		// Check for secrets and configmaps in env vars
		for _, env := range container.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.SecretKeyRef != nil {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("Container '%s' references secret '%s' - "+
							"ensure it's migrated to Parameter Store",
							container.Name, env.ValueFrom.SecretKeyRef.Name))
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("Container '%s' references configmap '%s' - "+
							"ensure it's migrated to Parameter Store",
							container.Name, env.ValueFrom.ConfigMapKeyRef.Name))
				}
			}
		}
	}

	// Check service account
	if pod.Spec.ServiceAccountName != "" && pod.Spec.ServiceAccountName != "default" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Pod uses service account '%s' - "+
				"ensure appropriate IAM roles are configured for ECS",
				pod.Spec.ServiceAccountName))
	}

	// Check for volume types that need attention
	for _, volume := range pod.Spec.Volumes {
		if volume.HostPath != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Volume '%s' uses hostPath - "+
					"ensure the path is available in ECS", volume.Name))
		}
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
		fmt.Printf("Summary: %d pods cannot be converted to ECS tasks without modifications.\n",
			summary.FailedPods)
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
