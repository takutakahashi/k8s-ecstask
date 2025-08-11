package ecs

import (
	corev1 "k8s.io/api/core/v1"
)

// XPodSpec represents the xpod specification
type XPodSpec struct {
	// Basic pod spec fields we support
	Containers       []corev1.Container       `json:"containers"`
	InitContainers   []corev1.Container       `json:"initContainers,omitempty"`
	Volumes          []corev1.Volume          `json:"volumes,omitempty"`
	ServiceAccount   string                   `json:"serviceAccountName,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	RestartPolicy    corev1.RestartPolicy     `json:"restartPolicy,omitempty"`
	
	// ECS-specific metadata
	Family              string            `json:"family"`
	TaskRoleArn         string            `json:"taskRoleArn,omitempty"`
	ExecutionRoleArn    string            `json:"executionRoleArn,omitempty"`
	NetworkMode         string            `json:"networkMode,omitempty"`
	RequiresCompatibilities []string      `json:"requiresCompatibilities,omitempty"`
	CPU                 string            `json:"cpu,omitempty"`
	Memory              string            `json:"memory,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

// ECSTaskDefinition represents the ECS task definition output
type ECSTaskDefinition struct {
	Family                   string                    `json:"family"`
	TaskRoleArn             string                    `json:"taskRoleArn,omitempty"`
	ExecutionRoleArn        string                    `json:"executionRoleArn,omitempty"`
	NetworkMode             string                    `json:"networkMode,omitempty"`
	RequiresCompatibilities []string                  `json:"requiresCompatibilities,omitempty"`
	CPU                     string                    `json:"cpu,omitempty"`
	Memory                  string                    `json:"memory,omitempty"`
	ContainerDefinitions    []ECSContainerDefinition  `json:"containerDefinitions"`
	Volumes                 []ECSVolume              `json:"volumes,omitempty"`
	Tags                    []ECSTag                 `json:"tags,omitempty"`
}

// ECSContainerDefinition represents an ECS container definition
type ECSContainerDefinition struct {
	Name             string                    `json:"name"`
	Image            string                    `json:"image"`
	CPU              int                       `json:"cpu,omitempty"`
	Memory           int                       `json:"memory,omitempty"`
	MemoryReservation int                      `json:"memoryReservation,omitempty"`
	Essential        bool                      `json:"essential"`
	PortMappings     []ECSPortMapping         `json:"portMappings,omitempty"`
	Environment      []ECSKeyValuePair        `json:"environment,omitempty"`
	Secrets          []ECSSecret              `json:"secrets,omitempty"`
	MountPoints      []ECSMountPoint          `json:"mountPoints,omitempty"`
	VolumesFrom      []ECSVolumeFrom          `json:"volumesFrom,omitempty"`
	LogConfiguration *ECSLogConfiguration     `json:"logConfiguration,omitempty"`
	Command          []string                 `json:"command,omitempty"`
	EntryPoint       []string                 `json:"entryPoint,omitempty"`
	WorkingDirectory string                   `json:"workingDirectory,omitempty"`
	User             string                   `json:"user,omitempty"`
}

// ECSPortMapping represents port mapping in ECS
type ECSPortMapping struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// ECSKeyValuePair represents environment variables
type ECSKeyValuePair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ECSSecret represents secrets from Parameter Store
type ECSSecret struct {
	Name      string `json:"name"`
	ValueFrom string `json:"valueFrom"`
}

// ECSMountPoint represents volume mount points
type ECSMountPoint struct {
	SourceVolume  string `json:"sourceVolume"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
}

// ECSVolumeFrom represents volumes from other containers
type ECSVolumeFrom struct {
	SourceContainer string `json:"sourceContainer"`
	ReadOnly        bool   `json:"readOnly,omitempty"`
}

// ECSLogConfiguration represents logging configuration
type ECSLogConfiguration struct {
	LogDriver string            `json:"logDriver"`
	Options   map[string]string `json:"options,omitempty"`
}

// ECSVolume represents ECS volume definitions
type ECSVolume struct {
	Name string         `json:"name"`
	Host *ECSHostVolume `json:"host,omitempty"`
}

// ECSHostVolume represents host volume configuration
type ECSHostVolume struct {
	SourcePath string `json:"sourcePath,omitempty"`
}

// ECSTag represents ECS resource tags
type ECSTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConversionOptions represents options for the conversion process
type ConversionOptions struct {
	// ParameterStorePrefix is the prefix for Parameter Store parameters
	ParameterStorePrefix string
	
	// DefaultLogDriver is the default log driver to use
	DefaultLogDriver string
	
	// DefaultLogOptions are the default log options
	DefaultLogOptions map[string]string
	
	// SkipUnsupportedFeatures will skip unsupported Kubernetes features
	SkipUnsupportedFeatures bool
	
	// DefaultExecutionRoleArn is used if not specified in the spec
	DefaultExecutionRoleArn string
	
	// DefaultTaskRoleArn is used if not specified in the spec
	DefaultTaskRoleArn string
}