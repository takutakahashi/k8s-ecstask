package ecs

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Converter handles the conversion from Kubernetes Pod spec to ECS task definition
type Converter struct {
	options ConversionOptions
}

// NewConverter creates a new converter with the given options
func NewConverter(options ConversionOptions) *Converter {
	// Set default values
	if options.DefaultLogDriver == "" {
		options.DefaultLogDriver = "awslogs"
	}
	if options.DefaultLogOptions == nil {
		options.DefaultLogOptions = map[string]string{
			"awslogs-group":  "/ecs/task",
			"awslogs-region": "us-east-1",
		}
	}
	if options.ParameterStorePrefix == "" {
		options.ParameterStorePrefix = "/pods"
	}

	return &Converter{
		options: options,
	}
}

// Convert converts a Kubernetes Pod to an ECS task definition
func (c *Converter) Convert(
	podSpec *corev1.PodSpec,
	ecsConfig *ECSConfig,
	namespace string,
) (*ECSTaskDefinition, error) {
	return c.ConvertPod(nil, podSpec, ecsConfig, namespace)
}

// ConvertPod converts a Kubernetes Pod (with metadata) to an ECS task definition
func (c *Converter) ConvertPod(
	pod *corev1.Pod,
	podSpec *corev1.PodSpec,
	ecsConfig *ECSConfig,
	namespace string,
) (*ECSTaskDefinition, error) {
	taskDef := &ECSTaskDefinition{
		Family:                  ecsConfig.Family,
		TaskRoleArn:             c.getTaskRoleArn(ecsConfig),
		ExecutionRoleArn:        c.getExecutionRoleArn(ecsConfig),
		NetworkMode:             c.getNetworkMode(ecsConfig),
		RequiresCompatibilities: c.getRequiresCompatibilities(ecsConfig, pod),
		CPU:                     ecsConfig.CPU,
		Memory:                  ecsConfig.Memory,
	}

	// Convert containers
	containerDefs, err := c.convertContainers(podSpec.Containers, namespace, false)
	if err != nil {
		return nil, fmt.Errorf("failed to convert containers: %w", err)
	}
	taskDef.ContainerDefinitions = containerDefs

	// Convert init containers (ECS doesn't support init containers directly)
	if len(podSpec.InitContainers) > 0 && !c.options.SkipUnsupportedFeatures {
		return nil, fmt.Errorf("init containers are not supported in ECS")
	}

	// Convert volumes
	volumes, err := c.convertVolumes(podSpec.Volumes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert volumes: %w", err)
	}
	taskDef.Volumes = volumes

	// Convert tags
	if len(ecsConfig.Tags) > 0 {
		tags := make([]ECSTag, 0, len(ecsConfig.Tags))
		for key, value := range ecsConfig.Tags {
			tags = append(tags, ECSTag{
				Key:   key,
				Value: value,
			})
		}
		taskDef.Tags = tags
	}

	return taskDef, nil
}

func (c *Converter) getTaskRoleArn(ecsConfig *ECSConfig) string {
	if ecsConfig.TaskRoleArn != "" {
		return ecsConfig.TaskRoleArn
	}
	return c.options.DefaultTaskRoleArn
}

func (c *Converter) getExecutionRoleArn(ecsConfig *ECSConfig) string {
	if ecsConfig.ExecutionRoleArn != "" {
		return ecsConfig.ExecutionRoleArn
	}
	return c.options.DefaultExecutionRoleArn
}

func (c *Converter) getNetworkMode(ecsConfig *ECSConfig) string {
	if ecsConfig.NetworkMode != "" {
		return ecsConfig.NetworkMode
	}
	return "awsvpc"
}

func (c *Converter) getRequiresCompatibilities(ecsConfig *ECSConfig, pod *corev1.Pod) []string {
	if len(ecsConfig.RequiresCompatibilities) > 0 {
		return ecsConfig.RequiresCompatibilities
	}

	// Check for annotation-based compatibility requirements
	if pod != nil && pod.Annotations != nil {
		if compatibilities, exists := pod.Annotations["ecs.takutakahashi.dev/requires-compatibilities"]; exists {
			return strings.Split(compatibilities, ",")
		}
	}

	return []string{"FARGATE"}
}

func (c *Converter) convertContainers(
	containers []corev1.Container,
	namespace string,
	isInit bool,
) ([]ECSContainerDefinition, error) {
	containerDefs := make([]ECSContainerDefinition, 0, len(containers))

	for _, container := range containers {
		containerDef, err := c.convertContainer(container, namespace, !isInit)
		if err != nil {
			return nil, fmt.Errorf("failed to convert container %s: %w", container.Name, err)
		}
		containerDefs = append(containerDefs, *containerDef)
	}

	return containerDefs, nil
}

func (c *Converter) convertContainer(
	container corev1.Container,
	namespace string,
	essential bool,
) (*ECSContainerDefinition, error) {
	containerDef := &ECSContainerDefinition{
		Name:      container.Name,
		Image:     container.Image,
		Essential: essential,
	}

	// Convert resource requirements
	c.convertResources(container, containerDef)

	// Convert port mappings
	if len(container.Ports) > 0 {
		portMappings := make([]ECSPortMapping, 0, len(container.Ports))
		for _, port := range container.Ports {
			portMapping := ECSPortMapping{
				ContainerPort: int(port.ContainerPort),
				Protocol:      strings.ToLower(string(port.Protocol)),
			}
			if port.HostPort != 0 {
				portMapping.HostPort = int(port.HostPort)
			}
			portMappings = append(portMappings, portMapping)
		}
		containerDef.PortMappings = portMappings
	}

	// Convert environment variables and secrets
	if err := c.convertEnvironment(container, containerDef, namespace); err != nil {
		return nil, fmt.Errorf("failed to convert environment: %w", err)
	}

	// Convert commands
	if len(container.Command) > 0 {
		containerDef.EntryPoint = container.Command
	}
	if len(container.Args) > 0 {
		containerDef.Command = container.Args
	}

	// Convert working directory
	if container.WorkingDir != "" {
		containerDef.WorkingDirectory = container.WorkingDir
	}

	// Convert volume mounts
	if len(container.VolumeMounts) > 0 {
		mountPoints := make([]ECSMountPoint, 0, len(container.VolumeMounts))
		for _, mount := range container.VolumeMounts {
			mountPoint := ECSMountPoint{
				SourceVolume:  mount.Name,
				ContainerPath: mount.MountPath,
				ReadOnly:      mount.ReadOnly,
			}
			mountPoints = append(mountPoints, mountPoint)
		}
		containerDef.MountPoints = mountPoints
	}

	// Set default log configuration
	containerDef.LogConfiguration = &ECSLogConfiguration{
		LogDriver: c.options.DefaultLogDriver,
		Options:   c.options.DefaultLogOptions,
	}

	return containerDef, nil
}

func (c *Converter) convertResources(container corev1.Container, containerDef *ECSContainerDefinition) {
	if container.Resources.Limits != nil {
		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
			cpuMillis := cpu.MilliValue()
			containerDef.CPU = int(cpuMillis)
		}

		if memory := container.Resources.Limits.Memory(); memory != nil {
			memoryMB := memory.Value() / (1024 * 1024)
			containerDef.Memory = int(memoryMB)
		}
	}

	if container.Resources.Requests != nil {
		if memory := container.Resources.Requests.Memory(); memory != nil {
			memoryMB := memory.Value() / (1024 * 1024)
			containerDef.MemoryReservation = int(memoryMB)
		}
	}
}

func (c *Converter) convertEnvironment(
	container corev1.Container,
	containerDef *ECSContainerDefinition,
	namespace string,
) error {
	var environment []ECSKeyValuePair
	var secrets []ECSSecret

	for _, env := range container.Env {
		if env.ValueFrom != nil {
			// Handle secrets and config maps
			if env.ValueFrom.SecretKeyRef != nil {
				secret := ECSSecret{
					Name: env.Name,
					ValueFrom: c.getParameterStorePathForSecret(
						namespace,
						env.ValueFrom.SecretKeyRef.Name,
						env.ValueFrom.SecretKeyRef.Key,
					),
				}
				secrets = append(secrets, secret)
			} else if env.ValueFrom.ConfigMapKeyRef != nil {
				secret := ECSSecret{
					Name: env.Name,
					ValueFrom: c.getParameterStorePathForConfigMap(
						namespace,
						env.ValueFrom.ConfigMapKeyRef.Name,
						env.ValueFrom.ConfigMapKeyRef.Key,
					),
				}
				secrets = append(secrets, secret)
			} else if env.ValueFrom.FieldRef != nil || env.ValueFrom.ResourceFieldRef != nil {
				if !c.options.SkipUnsupportedFeatures {
					return fmt.Errorf("field references are not supported in ECS")
				}
			}
		} else {
			// Regular environment variable
			envVar := ECSKeyValuePair{
				Name:  env.Name,
				Value: env.Value,
			}
			environment = append(environment, envVar)
		}
	}

	if len(environment) > 0 {
		containerDef.Environment = environment
	}
	if len(secrets) > 0 {
		containerDef.Secrets = secrets
	}

	return nil
}

func (c *Converter) getParameterStorePathForSecret(namespace, secretName, key string) string {
	if namespace == "" {
		namespace = "default"
	}
	return fmt.Sprintf("%s/%s/secrets/%s/%s", c.options.ParameterStorePrefix, namespace, secretName, key)
}

func (c *Converter) getParameterStorePathForConfigMap(namespace, configMapName, key string) string {
	if namespace == "" {
		namespace = "default"
	}
	return fmt.Sprintf("%s/%s/configmaps/%s/%s", c.options.ParameterStorePrefix, namespace, configMapName, key)
}

func (c *Converter) convertVolumes(volumes []corev1.Volume) ([]ECSVolume, error) {
	var ecsVolumes []ECSVolume

	for _, volume := range volumes {
		if volume.HostPath != nil {
			ecsVolume := ECSVolume{
				Name: volume.Name,
				Host: &ECSHostVolume{
					SourcePath: volume.HostPath.Path,
				},
			}
			ecsVolumes = append(ecsVolumes, ecsVolume)
		} else if volume.EmptyDir != nil {
			// ECS doesn't support emptyDir, use host volume instead
			ecsVolume := ECSVolume{
				Name: volume.Name,
				Host: &ECSHostVolume{},
			}
			ecsVolumes = append(ecsVolumes, ecsVolume)
		} else if volume.Secret != nil || volume.ConfigMap != nil {
			if !c.options.SkipUnsupportedFeatures {
				return nil, fmt.Errorf("secret/configmap volumes are not supported in ECS, use Parameter Store instead")
			}
		} else {
			if !c.options.SkipUnsupportedFeatures {
				return nil, fmt.Errorf("unsupported volume type for volume %s", volume.Name)
			}
		}
	}

	return ecsVolumes, nil
}
