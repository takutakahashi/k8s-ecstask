package ecs

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// XPodSpecYAML represents the xpod specification for YAML parsing
type XPodSpecYAML struct {
	// Basic pod spec fields we support
	Containers       []ContainerYAML              `yaml:"containers"`
	InitContainers   []ContainerYAML              `yaml:"initContainers,omitempty"`
	Volumes          []corev1.Volume              `yaml:"volumes,omitempty"`
	ServiceAccount   string                       `yaml:"serviceAccountName,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `yaml:"imagePullSecrets,omitempty"`
	RestartPolicy    corev1.RestartPolicy         `yaml:"restartPolicy,omitempty"`
	
	// ECS-specific metadata
	Family              string            `yaml:"family"`
	TaskRoleArn         string            `yaml:"taskRoleArn,omitempty"`
	ExecutionRoleArn    string            `yaml:"executionRoleArn,omitempty"`
	NetworkMode         string            `yaml:"networkMode,omitempty"`
	RequiresCompatibilities []string      `yaml:"requiresCompatibilities,omitempty"`
	CPU                 string            `yaml:"cpu,omitempty"`
	Memory              string            `yaml:"memory,omitempty"`
	Tags                map[string]string `yaml:"tags,omitempty"`
}

// ContainerYAML represents a container for YAML parsing
type ContainerYAML struct {
	Name            string                    `yaml:"name"`
	Image           string                    `yaml:"image"`
	Command         []string                  `yaml:"command,omitempty"`
	Args            []string                  `yaml:"args,omitempty"`
	WorkingDir      string                    `yaml:"workingDir,omitempty"`
	Ports           []corev1.ContainerPort    `yaml:"ports,omitempty"`
	Env             []corev1.EnvVar           `yaml:"env,omitempty"`
	Resources       ResourceRequirementsYAML  `yaml:"resources,omitempty"`
	VolumeMounts    []corev1.VolumeMount      `yaml:"volumeMounts,omitempty"`
}

// ResourceRequirementsYAML represents resource requirements for YAML parsing
type ResourceRequirementsYAML struct {
	Limits   map[string]string `yaml:"limits,omitempty"`
	Requests map[string]string `yaml:"requests,omitempty"`
}

// ToXPodSpec converts XPodSpecYAML to XPodSpec
func (y *XPodSpecYAML) ToXPodSpec() (*XPodSpec, error) {
	spec := &XPodSpec{
		Family:                  y.Family,
		TaskRoleArn:             y.TaskRoleArn,
		ExecutionRoleArn:        y.ExecutionRoleArn,
		NetworkMode:             y.NetworkMode,
		RequiresCompatibilities: y.RequiresCompatibilities,
		CPU:                     y.CPU,
		Memory:                  y.Memory,
		Tags:                    y.Tags,
		Volumes:                 y.Volumes,
		ServiceAccount:          y.ServiceAccount,
		ImagePullSecrets:        y.ImagePullSecrets,
		RestartPolicy:           y.RestartPolicy,
	}

	// Convert containers
	containers, err := convertContainersYAML(y.Containers)
	if err != nil {
		return nil, err
	}
	spec.Containers = containers

	// Convert init containers
	if len(y.InitContainers) > 0 {
		initContainers, err := convertContainersYAML(y.InitContainers)
		if err != nil {
			return nil, err
		}
		spec.InitContainers = initContainers
	}

	return spec, nil
}

func convertContainersYAML(containers []ContainerYAML) ([]corev1.Container, error) {
	var result []corev1.Container

	for _, c := range containers {
		container := corev1.Container{
			Name:         c.Name,
			Image:        c.Image,
			Command:      c.Command,
			Args:         c.Args,
			WorkingDir:   c.WorkingDir,
			Ports:        c.Ports,
			Env:          c.Env,
			VolumeMounts: c.VolumeMounts,
		}

		// Convert resources
		if c.Resources.Limits != nil || c.Resources.Requests != nil {
			resources := corev1.ResourceRequirements{}
			
			if c.Resources.Limits != nil {
				limits := make(corev1.ResourceList)
				for k, v := range c.Resources.Limits {
					quantity, err := resource.ParseQuantity(v)
					if err != nil {
						return nil, err
					}
					limits[corev1.ResourceName(k)] = quantity
				}
				resources.Limits = limits
			}
			
			if c.Resources.Requests != nil {
				requests := make(corev1.ResourceList)
				for k, v := range c.Resources.Requests {
					quantity, err := resource.ParseQuantity(v)
					if err != nil {
						return nil, err
					}
					requests[corev1.ResourceName(k)] = quantity
				}
				resources.Requests = requests
			}
			
			container.Resources = resources
		}

		result = append(result, container)
	}

	return result, nil
}