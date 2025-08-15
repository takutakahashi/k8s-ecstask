package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/takutakahashi/k8s-ecstask/pkg/ecs"
)

func main() {
	var (
		inputFile  = flag.String("input", "", "Input YAML file containing Kubernetes Pod specification")
		outputFile = flag.String("output", "", "Output JSON file for ECS task definition (default: stdout)")
		family     = flag.String("family", "", "ECS task definition family name (required)")
		namespace  = flag.String("namespace", "",
			"Kubernetes namespace (extracted from Pod metadata if not specified)")
		parameterStorePrefix = flag.String("parameter-store-prefix", "/pods", "Prefix for Parameter Store parameters")
		executionRoleArn     = flag.String("execution-role-arn", "", "ECS execution role ARN")
		taskRoleArn          = flag.String("task-role-arn", "", "ECS task role ARN")
		networkMode          = flag.String("network-mode", "awsvpc", "ECS network mode")
		cpu                  = flag.String("cpu", "", "Task-level CPU allocation")
		memory               = flag.String("memory", "", "Task-level memory allocation")
		logGroup             = flag.String("log-group", "/ecs/pods", "CloudWatch log group")
		logRegion            = flag.String("log-region", "us-east-1", "AWS region for logs")
		skipUnsupported      = flag.Bool("skip-unsupported", true, "Skip unsupported Kubernetes features")
	)
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -input <yaml-file> -family <family-name> [options]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *family == "" {
		fmt.Fprintf(os.Stderr, "Error: -family is required\n")
		os.Exit(1)
	}

	// Read input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Parse YAML as Kubernetes Pod using Kubernetes YAML decoder
	var pod corev1.Pod
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	if err := decoder.Decode(&pod); err != nil {
		log.Fatalf("Failed to parse Kubernetes Pod YAML: %v", err)
	}

	// Create ECS configuration
	ecsConfig := &ecs.ECSConfig{
		Family:                  *family,
		ExecutionRoleArn:        *executionRoleArn,
		TaskRoleArn:             *taskRoleArn,
		NetworkMode:             *networkMode,
		RequiresCompatibilities: nil, // Let annotation logic determine compatibility
		CPU:                     *cpu,
		Memory:                  *memory,
	}

	// Create converter
	options := ecs.ConversionOptions{
		ParameterStorePrefix:    *parameterStorePrefix,
		DefaultExecutionRoleArn: *executionRoleArn,
		DefaultTaskRoleArn:      *taskRoleArn,
		DefaultLogDriver:        "awslogs",
		DefaultLogOptions: map[string]string{
			"awslogs-group":  *logGroup,
			"awslogs-region": *logRegion,
		},
		SkipUnsupportedFeatures: *skipUnsupported,
	}

	converter := ecs.NewConverter(options)

	// Determine namespace
	ns := *namespace
	if ns == "" {
		ns = pod.Namespace
	}
	if ns == "" {
		ns = "default"
	}

	// Convert to ECS task definition
	taskDef, err := converter.ConvertPod(&pod, &pod.Spec, ecsConfig, ns)
	if err != nil {
		log.Fatalf("Failed to convert: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(taskDef, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Write output
	var output io.Writer = os.Stdout
	if *outputFile != "" {
		file, err := os.Create(*outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("Failed to close output file: %v", err)
			}
		}()
		output = file
	}

	if _, err := output.Write(jsonData); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	if _, err := fmt.Fprintln(output); err != nil {
		log.Printf("Failed to write newline: %v", err)
	}

	if *outputFile != "" {
		fmt.Printf("ECS task definition written to %s\n", *outputFile)
	}
}
