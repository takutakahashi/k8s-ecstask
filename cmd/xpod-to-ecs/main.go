package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/takutakahashi/k8s-ecstask/pkg/ecs"
)

func main() {
	var (
		inputFile            = flag.String("input", "", "Input YAML file containing XPod specification")
		outputFile           = flag.String("output", "", "Output JSON file for ECS task definition (default: stdout)")
		parameterStorePrefix = flag.String("parameter-store-prefix", "/xpod", "Prefix for Parameter Store parameters")
		executionRoleArn     = flag.String("execution-role-arn", "", "Default execution role ARN")
		taskRoleArn          = flag.String("task-role-arn", "", "Default task role ARN")
		logGroup             = flag.String("log-group", "/ecs/task", "CloudWatch log group")
		logRegion            = flag.String("log-region", "us-east-1", "AWS region for logs")
		skipUnsupported      = flag.Bool("skip-unsupported", true, "Skip unsupported Kubernetes features")
	)
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -input <yaml-file> [options]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Read input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Parse YAML
	var xpodSpecYAML ecs.XPodSpecYAML
	if err := yaml.Unmarshal(data, &xpodSpecYAML); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Convert to XPodSpec
	xpodSpec, err := xpodSpecYAML.ToXPodSpec()
	if err != nil {
		log.Fatalf("Failed to convert YAML spec: %v", err)
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

	// Convert to ECS task definition
	taskDef, err := converter.Convert(xpodSpec)
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
		defer file.Close()
		output = file
	}

	if _, err := output.Write(jsonData); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	fmt.Fprintln(output)
	
	if *outputFile != "" {
		fmt.Printf("ECS task definition written to %s\n", *outputFile)
	}
}