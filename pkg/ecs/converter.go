package ecs

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Converter handles the conversion from XPod spec to ECS task definition
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
		options.ParameterStorePrefix = "/xpod"
	}
	
	return &Converter{
		options: options,
	}
}

// Convert converts an XPodSpec to an ECS task definition
func (c *Converter) Convert(spec *XPodSpec) (*ECSTaskDefinition, error) {
	taskDef := &ECSTaskDefinition{
		Family:                  spec.Family,
		TaskRoleArn:            c.getTaskRoleArn(spec),
		ExecutionRoleArn:       c.getExecutionRoleArn(spec),
		NetworkMode:            c.getNetworkMode(spec),
		RequiresCompatibilities: c.getRequiresCompatibilities(spec),
		CPU:                    spec.CPU,
		Memory:                 spec.Memory,
	}

	// Convert containers
	containerDefs, err := c.convertContainers(spec.Containers, false)
	if err != nil {
		return nil, fmt.Errorf("failed to convert containers: %w", err)
	}
	taskDef.ContainerDefinitions = containerDefs

	// Convert init containers (ECS doesn't support init containers directly)
	if len(spec.InitContainers) > 0 && !c.options.SkipUnsupportedFeatures {
		return nil, fmt.Errorf("init containers are not supported in ECS")
	}

	// Convert volumes
	volumes, err := c.convertVolumes(spec.Volumes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert volumes: %w", err)
	}
	taskDef.Volumes = volumes

	// Convert tags
	if len(spec.Tags) > 0 {
		tags := make([]ECSTag, 0, len(spec.Tags))
		for key, value := range spec.Tags {
			tags = append(tags, ECSTag{
				Key:   key,
				Value: value,
			})
		}
		taskDef.Tags = tags
	}

	return taskDef, nil
}

func (c *Converter) getTaskRoleArn(spec *XPodSpec) string {
	if spec.TaskRoleArn != "" {
		return spec.TaskRoleArn
	}
	return c.options.DefaultTaskRoleArn
}

func (c *Converter) getExecutionRoleArn(spec *XPodSpec) string {
	if spec.ExecutionRoleArn != "" {
		return spec.ExecutionRoleArn
	}
	return c.options.DefaultExecutionRoleArn
}

func (c *Converter) getNetworkMode(spec *XPodSpec) string {
	if spec.NetworkMode != "" {
		return spec.NetworkMode
	}
	return "awsvpc"
}

func (c *Converter) getRequiresCompatibilities(spec *XPodSpec) []string {
	if len(spec.RequiresCompatibilities) > 0 {
		return spec.RequiresCompatibilities
	}
	return []string{"FARGATE"}
}

func (c *Converter) convertContainers(containers []corev1.Container, isInit bool) ([]ECSContainerDefinition, error) {
	var containerDefs []ECSContainerDefinition

	for _, container := range containers {
		containerDef, err := c.convertContainer(container, !isInit)
		if err != nil {
			return nil, fmt.Errorf("failed to convert container %s: %w", container.Name, err)
		}
		containerDefs = append(containerDefs, *containerDef)
	}

	return containerDefs, nil
}

func (c *Converter) convertContainer(container corev1.Container, essential bool) (*ECSContainerDefinition, error) {
	containerDef := &ECSContainerDefinition{
		Name:      container.Name,
		Image:     container.Image,
		Essential: essential,
	}

	// Convert resource requirements
	if err := c.convertResources(container, containerDef); err != nil {
		return nil, fmt.Errorf("failed to convert resources: %w", err)
	}

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
	if err := c.convertEnvironment(container, containerDef); err != nil {
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

func (c *Converter) convertResources(container corev1.Container, containerDef *ECSContainerDefinition) error {
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

	return nil
}

func (c *Converter) convertEnvironment(container corev1.Container, containerDef *ECSContainerDefinition) error {
	var environment []ECSKeyValuePair
	var secrets []ECSSecret

	for _, env := range container.Env {
		if env.ValueFrom != nil {
			// Handle secrets and config maps
			if env.ValueFrom.SecretKeyRef != nil {
				secret := ECSSecret{
					Name:      env.Name,
					ValueFrom: c.getParameterStorePathForSecret(env.ValueFrom.SecretKeyRef.Name, env.ValueFrom.SecretKeyRef.Key),
				}
				secrets = append(secrets, secret)
			} else if env.ValueFrom.ConfigMapKeyRef != nil {
				secret := ECSSecret{
					Name:      env.Name,
					ValueFrom: c.getParameterStorePathForConfigMap(env.ValueFrom.ConfigMapKeyRef.Name, env.ValueFrom.ConfigMapKeyRef.Key),
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

func (c *Converter) getParameterStorePathForSecret(secretName, key string) string {
	return fmt.Sprintf("%s/secrets/%s/%s", c.options.ParameterStorePrefix, secretName, key)
}

func (c *Converter) getParameterStorePathForConfigMap(configMapName, key string) string {
	return fmt.Sprintf("%s/configmaps/%s/%s", c.options.ParameterStorePrefix, configMapName, key)
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