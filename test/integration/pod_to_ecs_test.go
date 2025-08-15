package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPodToECSConversion(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "pod-to-ecs", "../../cmd/pod-to-ecs")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build pod-to-ecs binary: %v", err)
	}
	defer func() {
		if err := os.Remove("pod-to-ecs"); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	tests := []struct {
		name           string
		inputFile      string
		expectedChecks func(t *testing.T, taskDef map[string]interface{})
		additionalArgs []string
	}{
		{
			name:           "Simple Pod Conversion",
			inputFile:      "../fixtures/simple-pod-unquoted.yaml",
			expectedChecks: testSimplePod,
		},
		{
			name:           "Pod with Environment Variables",
			inputFile:      "../fixtures/pod-with-env-unquoted.yaml",
			expectedChecks: testPodWithEnv,
		},
		{
			name:           "Pod with Volumes",
			inputFile:      "../fixtures/pod-with-volumes-unquoted.yaml",
			expectedChecks: testPodWithVolumes,
		},
		{
			name:           "Pod with EXTERNAL annotation",
			inputFile:      "../fixtures/pod-with-external-annotation.yaml",
			expectedChecks: testPodWithExternal,
		},
		{
			name:           "Pod with mixed compatibility",
			inputFile:      "../fixtures/pod-with-mixed-compatibility.yaml",
			expectedChecks: testPodWithMixedCompatibility,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runTest(t, tt.inputFile, tt.expectedChecks, tt.additionalArgs)
		})
	}
}

func runTest(
	t *testing.T,
	inputFile string,
	expectedChecks func(t *testing.T, taskDef map[string]interface{}),
	additionalArgs []string,
) {
	// Create temp output file
	outputFile := filepath.Join(t.TempDir(), "task-def.json")

	// Prepare command arguments
	args := []string{
		"-input", inputFile,
		"-output", outputFile,
		"-family", "test-app",
		"-execution-role-arn", "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
		"-task-role-arn", "arn:aws:iam::123456789012:role/ecsTaskRole",
		"-cpu", "1024",
		"-memory", "2048",
	}
	args = append(args, additionalArgs...)

	// Run the conversion
	cmd := exec.Command("./pod-to-ecs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run pod-to-ecs: %v\nOutput: %s", err, output)
	}

	// Read and parse the output
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var taskDef map[string]interface{}
	if err := json.Unmarshal(data, &taskDef); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Run test-specific checks
	expectedChecks(t, taskDef)

	// Common checks (skip compatibility check for annotation tests)
	isAnnotationTest := strings.Contains(inputFile, "external-annotation") || 
		strings.Contains(inputFile, "mixed-compatibility")
	checkCommonFields(t, taskDef, !isAnnotationTest)
}

func checkCommonFields(t *testing.T, taskDef map[string]interface{}, checkDefaultCompatibility bool) {
	executionRole := "arn:aws:iam::123456789012:role/ecsTaskExecutionRole"
	if taskDef["executionRoleArn"] != executionRole {
		t.Errorf("Expected executionRoleArn %s, got %v", executionRole, taskDef["executionRoleArn"])
	}

	taskRole := "arn:aws:iam::123456789012:role/ecsTaskRole"
	if taskDef["taskRoleArn"] != taskRole {
		t.Errorf("Expected taskRoleArn %s, got %v", taskRole, taskDef["taskRoleArn"])
	}

	if taskDef["cpu"] != "1024" {
		t.Errorf("Expected cpu '1024', got %v", taskDef["cpu"])
	}
	if taskDef["memory"] != "2048" {
		t.Errorf("Expected memory '2048', got %v", taskDef["memory"])
	}
	if taskDef["networkMode"] != "awsvpc" {
		t.Errorf("Expected networkMode 'awsvpc', got %v", taskDef["networkMode"])
	}

	// Check requiresCompatibilities (default case)
	if checkDefaultCompatibility {
		compatibilities := taskDef["requiresCompatibilities"].([]interface{})
		if len(compatibilities) != 1 || compatibilities[0] != "FARGATE" {
			t.Errorf("Expected requiresCompatibilities ['FARGATE'], got %v", compatibilities)
		}
	}
}

func testSimplePod(t *testing.T, taskDef map[string]interface{}) {
	// Check family
	if taskDef["family"] != "test-app" {
		t.Errorf("Expected family 'test-app', got %v", taskDef["family"])
	}

	// Check container definitions
	containerDefs := taskDef["containerDefinitions"].([]interface{})
	if len(containerDefs) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containerDefs))
	}

	container := containerDefs[0].(map[string]interface{})
	if container["name"] != "app" {
		t.Errorf("Expected container name 'app', got %v", container["name"])
	}
	if container["image"] != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got %v", container["image"])
	}

	// Check CPU and Memory
	if container["cpu"] != float64(500) {
		t.Errorf("Expected cpu 500, got %v", container["cpu"])
	}
	if container["memory"] != float64(512) {
		t.Errorf("Expected memory 512, got %v", container["memory"])
	}

	// Check port mappings
	portMappings := container["portMappings"].([]interface{})
	if len(portMappings) != 1 {
		t.Fatalf("Expected 1 port mapping, got %d", len(portMappings))
	}
	port := portMappings[0].(map[string]interface{})
	if port["containerPort"] != float64(80) {
		t.Errorf("Expected containerPort 80, got %v", port["containerPort"])
	}
}

func testPodWithEnv(t *testing.T, taskDef map[string]interface{}) {
	containerDefs := taskDef["containerDefinitions"].([]interface{})
	container := containerDefs[0].(map[string]interface{})

	// Check environment variables
	env := container["environment"].([]interface{})
	if len(env) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(env))
	}
	envVar := env[0].(map[string]interface{})
	if envVar["name"] != "DATABASE_URL" {
		t.Errorf("Expected env name 'DATABASE_URL', got %v", envVar["name"])
	}

	// Check secrets
	secrets := container["secrets"].([]interface{})
	if len(secrets) != 2 {
		t.Fatalf("Expected 2 secrets, got %d", len(secrets))
	}

	// Check first secret (from K8s secret)
	secret1 := secrets[0].(map[string]interface{})
	if secret1["name"] != "API_KEY" {
		t.Errorf("Expected secret name 'API_KEY', got %v", secret1["name"])
	}
	expectedPath1 := "/pods/test-namespace/secrets/api-secret/api-key"
	if secret1["valueFrom"] != expectedPath1 {
		t.Errorf("Expected valueFrom '%s', got %v", expectedPath1, secret1["valueFrom"])
	}

	// Check second secret (from ConfigMap)
	secret2 := secrets[1].(map[string]interface{})
	if secret2["name"] != "CONFIG_VALUE" {
		t.Errorf("Expected secret name 'CONFIG_VALUE', got %v", secret2["name"])
	}
	expectedPath2 := "/pods/test-namespace/configmaps/app-config/config-value"
	if secret2["valueFrom"] != expectedPath2 {
		t.Errorf("Expected valueFrom '%s', got %v", expectedPath2, secret2["valueFrom"])
	}
}

func testPodWithVolumes(t *testing.T, taskDef map[string]interface{}) {
	// Check volumes
	volumes := taskDef["volumes"].([]interface{})
	if len(volumes) != 2 {
		t.Fatalf("Expected 2 volumes, got %d", len(volumes))
	}

	// Check emptyDir volume (converted to host volume)
	vol1 := volumes[0].(map[string]interface{})
	if vol1["name"] != "data-volume" {
		t.Errorf("Expected volume name 'data-volume', got %v", vol1["name"])
	}
	if vol1["host"] == nil {
		t.Error("Expected host volume for emptyDir")
	}

	// Check hostPath volume
	vol2 := volumes[1].(map[string]interface{})
	if vol2["name"] != "config-volume" {
		t.Errorf("Expected volume name 'config-volume', got %v", vol2["name"])
	}
	host := vol2["host"].(map[string]interface{})
	if host["sourcePath"] != "/etc/config" {
		t.Errorf("Expected sourcePath '/etc/config', got %v", host["sourcePath"])
	}

	// Check container mount points
	containerDefs := taskDef["containerDefinitions"].([]interface{})
	container := containerDefs[0].(map[string]interface{})
	mountPoints := container["mountPoints"].([]interface{})
	if len(mountPoints) != 2 {
		t.Fatalf("Expected 2 mount points, got %d", len(mountPoints))
	}

	// Check command and args
	entryPoint := container["entryPoint"].([]interface{})
	if len(entryPoint) != 1 || entryPoint[0] != "/bin/sh" {
		t.Errorf("Expected entryPoint ['/bin/sh'], got %v", entryPoint)
	}
	command := container["command"].([]interface{})
	if len(command) != 2 || command[0] != "-c" ||
		command[1] != "echo hello" {
		t.Errorf("Expected command ['-c', 'echo hello'], got %v", command)
	}

	// Check working directory
	if container["workingDirectory"] != "/app" {
		t.Errorf("Expected workingDirectory '/app', got %v", container["workingDirectory"])
	}
}

func testPodWithExternal(t *testing.T, taskDef map[string]interface{}) {
	// Check that EXTERNAL compatibility is set via annotation
	compatibilities := taskDef["requiresCompatibilities"].([]interface{})
	if len(compatibilities) != 1 || compatibilities[0] != "EXTERNAL" {
		t.Errorf("Expected requiresCompatibilities ['EXTERNAL'], got %v", compatibilities)
	}

	// Check basic conversion works
	containerDefs := taskDef["containerDefinitions"].([]interface{})
	if len(containerDefs) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containerDefs))
	}

	container := containerDefs[0].(map[string]interface{})
	if container["name"] != "app" {
		t.Errorf("Expected container name 'app', got %v", container["name"])
	}
	if container["image"] != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got %v", container["image"])
	}
}

func testPodWithMixedCompatibility(t *testing.T, taskDef map[string]interface{}) {
	// Check that mixed compatibility is set via annotation
	compatibilities := taskDef["requiresCompatibilities"].([]interface{})
	if len(compatibilities) != 2 {
		t.Fatalf("Expected 2 compatibility modes, got %d", len(compatibilities))
	}

	expectedCompat := map[string]bool{"FARGATE": false, "EXTERNAL": false}
	for _, compat := range compatibilities {
		compatStr := compat.(string)
		if _, exists := expectedCompat[compatStr]; exists {
			expectedCompat[compatStr] = true
		} else {
			t.Errorf("Unexpected compatibility mode: %s", compatStr)
		}
	}

	for mode, found := range expectedCompat {
		if !found {
			t.Errorf("Missing compatibility mode: %s", mode)
		}
	}
}
