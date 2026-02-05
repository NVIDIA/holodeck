/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package describe

import (
	"fmt"
	"strings"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/output"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log          *logger.FunLogger
	cachePath    string
	outputFormat string
}

// DescribeOutput represents the full instance description for JSON/YAML output
type DescribeOutput struct {
	Instance     InstanceInfo      `json:"instance" yaml:"instance"`
	Provider     ProviderInfo      `json:"provider" yaml:"provider"`
	Cluster      *ClusterInfo      `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	Components   ComponentsInfo    `json:"components" yaml:"components"`
	Status       StatusInfo        `json:"status" yaml:"status"`
	AWSResources *AWSResourcesInfo `json:"awsResources,omitempty" yaml:"awsResources,omitempty"`
}

// InstanceInfo contains basic instance metadata
type InstanceInfo struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	Age         string    `json:"age" yaml:"age"`
	CacheFile   string    `json:"cacheFile" yaml:"cacheFile"`
	Provisioned bool      `json:"provisioned" yaml:"provisioned"`
}

// ProviderInfo contains provider configuration
type ProviderInfo struct {
	Type     string `json:"type" yaml:"type"`
	Region   string `json:"region,omitempty" yaml:"region,omitempty"`
	Username string `json:"username" yaml:"username"`
	KeyName  string `json:"keyName" yaml:"keyName"`
}

// ClusterInfo contains cluster configuration
type ClusterInfo struct {
	Region            string             `json:"region" yaml:"region"`
	ControlPlane      ControlPlaneInfo   `json:"controlPlane" yaml:"controlPlane"`
	Workers           *WorkersInfo       `json:"workers,omitempty" yaml:"workers,omitempty"`
	HighAvailability  *HAInfo            `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Phase             string             `json:"phase,omitempty" yaml:"phase,omitempty"`
	TotalNodes        int32              `json:"totalNodes,omitempty" yaml:"totalNodes,omitempty"`
	ReadyNodes        int32              `json:"readyNodes,omitempty" yaml:"readyNodes,omitempty"`
	Endpoint          string             `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	LoadBalancerDNS   string             `json:"loadBalancerDNS,omitempty" yaml:"loadBalancerDNS,omitempty"`
	Nodes             []NodeInfo         `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

// ControlPlaneInfo contains control plane configuration
type ControlPlaneInfo struct {
	Count        int32  `json:"count" yaml:"count"`
	InstanceType string `json:"instanceType" yaml:"instanceType"`
	Dedicated    bool   `json:"dedicated" yaml:"dedicated"`
}

// WorkersInfo contains worker pool configuration
type WorkersInfo struct {
	Count        int32  `json:"count" yaml:"count"`
	InstanceType string `json:"instanceType" yaml:"instanceType"`
}

// HAInfo contains high availability configuration
type HAInfo struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	EtcdTopology     string `json:"etcdTopology,omitempty" yaml:"etcdTopology,omitempty"`
	LoadBalancerType string `json:"loadBalancerType,omitempty" yaml:"loadBalancerType,omitempty"`
}

// NodeInfo contains individual node information
type NodeInfo struct {
	Name       string `json:"name" yaml:"name"`
	Role       string `json:"role" yaml:"role"`
	InstanceID string `json:"instanceId,omitempty" yaml:"instanceId,omitempty"`
	PublicIP   string `json:"publicIP,omitempty" yaml:"publicIP,omitempty"`
	PrivateIP  string `json:"privateIP,omitempty" yaml:"privateIP,omitempty"`
	Phase      string `json:"phase" yaml:"phase"`
}

// ComponentsInfo contains installed component information
type ComponentsInfo struct {
	Kernel           *KernelInfo           `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	NVIDIADriver     *NVIDIADriverInfo     `json:"nvidiaDriver,omitempty" yaml:"nvidiaDriver,omitempty"`
	ContainerRuntime *ContainerRuntimeInfo `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	ContainerToolkit *ContainerToolkitInfo `json:"containerToolkit,omitempty" yaml:"containerToolkit,omitempty"`
	Kubernetes       *KubernetesInfo       `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
}

// KernelInfo contains kernel configuration
type KernelInfo struct {
	Version string `json:"version" yaml:"version"`
}

// NVIDIADriverInfo contains NVIDIA driver configuration
type NVIDIADriverInfo struct {
	Install bool   `json:"install" yaml:"install"`
	Branch  string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ContainerRuntimeInfo contains container runtime configuration
type ContainerRuntimeInfo struct {
	Install bool   `json:"install" yaml:"install"`
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ContainerToolkitInfo contains NVIDIA Container Toolkit configuration
type ContainerToolkitInfo struct {
	Install   bool   `json:"install" yaml:"install"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	EnableCDI bool   `json:"enableCDI" yaml:"enableCDI"`
}

// KubernetesInfo contains Kubernetes configuration
type KubernetesInfo struct {
	Install   bool   `json:"install" yaml:"install"`
	Installer string `json:"installer,omitempty" yaml:"installer,omitempty"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
}

// StatusInfo contains status and conditions
type StatusInfo struct {
	State      string          `json:"state" yaml:"state"`
	Conditions []ConditionInfo `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// ConditionInfo represents a status condition
type ConditionInfo struct {
	Type    string `json:"type" yaml:"type"`
	Status  string `json:"status" yaml:"status"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// AWSResourcesInfo contains AWS-specific resource information
type AWSResourcesInfo struct {
	InstanceID    string `json:"instanceId,omitempty" yaml:"instanceId,omitempty"`
	InstanceType  string `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	PublicDNS     string `json:"publicDNS,omitempty" yaml:"publicDNS,omitempty"`
	PublicIP      string `json:"publicIP,omitempty" yaml:"publicIP,omitempty"`
	PrivateIP     string `json:"privateIP,omitempty" yaml:"privateIP,omitempty"`
	VpcID         string `json:"vpcId,omitempty" yaml:"vpcId,omitempty"`
	SubnetID      string `json:"subnetId,omitempty" yaml:"subnetId,omitempty"`
	SecurityGroup string `json:"securityGroup,omitempty" yaml:"securityGroup,omitempty"`
	AMI           string `json:"ami,omitempty" yaml:"ami,omitempty"`
}

// NewCommand constructs the describe command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	describeCmd := cli.Command{
		Name:      "describe",
		Usage:     "Show detailed information about a Holodeck instance",
		ArgsUsage: "<instance-id>",
		Description: `Display comprehensive information about a Holodeck instance including:
- Instance metadata and status
- Provider configuration
- Cluster details (for multinode)
- Installed components and versions
- AWS resources (VPC, Security Groups, etc.)

Examples:
  # Describe an instance
  holodeck describe abc123

  # Output as JSON
  holodeck describe abc123 -o json

  # Output as YAML
  holodeck describe abc123 -o yaml`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "Output format: table, json, yaml (default: table)",
				Destination: &m.outputFormat,
				Value:       "table",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required")
			}
			return m.run(c.Args().Get(0))
		},
	}

	return &describeCmd
}

func (m command) run(instanceID string) error {
	// Get instance details
	manager := instances.NewManager(m.log, m.cachePath)
	instance, err := manager.GetInstance(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %v", err)
	}

	// Load environment
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](instance.CacheFile)
	if err != nil {
		return fmt.Errorf("failed to read environment: %v", err)
	}

	age := time.Since(instance.CreatedAt).Round(time.Second)

	// Build describe output
	describeOutput := m.buildDescribeOutput(instance, &env, age)

	// Create formatter and output
	formatter, err := output.NewFormatter(m.outputFormat)
	if err != nil {
		return fmt.Errorf("invalid output format %q, must be one of: %s", m.outputFormat, strings.Join(output.ValidFormats(), ", "))
	}

	if formatter.Format() == output.FormatTable {
		return m.printTableFormat(describeOutput)
	}

	return formatter.Print(describeOutput)
}

func (m command) buildDescribeOutput(instance *instances.Instance, env *v1alpha1.Environment, age time.Duration) *DescribeOutput {
	// Instance info
	output := &DescribeOutput{
		Instance: InstanceInfo{
			ID:          instance.ID,
			Name:        env.Name,
			CreatedAt:   instance.CreatedAt,
			Age:         age.String(),
			CacheFile:   instance.CacheFile,
			Provisioned: instance.Provisioned,
		},
		Provider: ProviderInfo{
			Type:     string(env.Spec.Provider),
			Username: env.Spec.Username,
			KeyName:  env.Spec.KeyName,
		},
	}

	// Set region based on provider type
	if env.Spec.Cluster != nil {
		output.Provider.Region = env.Spec.Cluster.Region
	} else {
		output.Provider.Region = env.Spec.Instance.Region
	}

	// Cluster info
	if env.Spec.Cluster != nil {
		output.Cluster = &ClusterInfo{
			Region: env.Spec.Cluster.Region,
			ControlPlane: ControlPlaneInfo{
				Count:        env.Spec.Cluster.ControlPlane.Count,
				InstanceType: env.Spec.Cluster.ControlPlane.InstanceType,
				Dedicated:    env.Spec.Cluster.ControlPlane.Dedicated,
			},
		}

		if env.Spec.Cluster.Workers != nil {
			output.Cluster.Workers = &WorkersInfo{
				Count:        env.Spec.Cluster.Workers.Count,
				InstanceType: env.Spec.Cluster.Workers.InstanceType,
			}
		}

		if env.Spec.Cluster.HighAvailability != nil && env.Spec.Cluster.HighAvailability.Enabled {
			output.Cluster.HighAvailability = &HAInfo{
				Enabled:          true,
				EtcdTopology:     string(env.Spec.Cluster.HighAvailability.EtcdTopology),
				LoadBalancerType: env.Spec.Cluster.HighAvailability.LoadBalancerType,
			}
		}

		if env.Status.Cluster != nil {
			output.Cluster.Phase = string(env.Status.Cluster.Phase)
			output.Cluster.TotalNodes = env.Status.Cluster.TotalNodes
			output.Cluster.ReadyNodes = env.Status.Cluster.ReadyNodes
			output.Cluster.Endpoint = env.Status.Cluster.ControlPlaneEndpoint
			output.Cluster.LoadBalancerDNS = env.Status.Cluster.LoadBalancerDNS

			for _, node := range env.Status.Cluster.Nodes {
				output.Cluster.Nodes = append(output.Cluster.Nodes, NodeInfo{
					Name:       node.Name,
					Role:       node.Role,
					InstanceID: node.InstanceID,
					PublicIP:   node.PublicIP,
					PrivateIP:  node.PrivateIP,
					Phase:      string(node.Phase),
				})
			}
		}
	}

	// Components
	if env.Spec.Kernel.Version != "" {
		output.Components.Kernel = &KernelInfo{
			Version: env.Spec.Kernel.Version,
		}
	}

	if env.Spec.NVIDIADriver.Install {
		output.Components.NVIDIADriver = &NVIDIADriverInfo{
			Install: true,
			Branch:  env.Spec.NVIDIADriver.Branch,
			Version: env.Spec.NVIDIADriver.Version,
		}
	}

	if env.Spec.ContainerRuntime.Install {
		output.Components.ContainerRuntime = &ContainerRuntimeInfo{
			Install: true,
			Name:    string(env.Spec.ContainerRuntime.Name),
			Version: env.Spec.ContainerRuntime.Version,
		}
	}

	if env.Spec.NVIDIAContainerToolkit.Install {
		output.Components.ContainerToolkit = &ContainerToolkitInfo{
			Install:   true,
			Source:    string(env.Spec.NVIDIAContainerToolkit.Source),
			Version:   env.Spec.NVIDIAContainerToolkit.Version,
			EnableCDI: env.Spec.NVIDIAContainerToolkit.EnableCDI,
		}
	}

	if env.Spec.Kubernetes.Install {
		k8sVersion := env.Spec.Kubernetes.KubernetesVersion
		if env.Spec.Kubernetes.Release != nil {
			k8sVersion = env.Spec.Kubernetes.Release.Version
		}
		output.Components.Kubernetes = &KubernetesInfo{
			Install:   true,
			Installer: env.Spec.Kubernetes.KubernetesInstaller,
			Version:   k8sVersion,
			Source:    string(env.Spec.Kubernetes.Source),
		}
	}

	// Status
	output.Status.State = instance.Status
	for _, cond := range env.Status.Conditions {
		output.Status.Conditions = append(output.Status.Conditions, ConditionInfo{
			Type:    cond.Type,
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	// AWS Resources (for single-node AWS)
	if env.Spec.Provider == v1alpha1.ProviderAWS && env.Spec.Cluster == nil {
		awsRes := &AWSResourcesInfo{
			InstanceType: env.Spec.Instance.Type,
		}
		if env.Spec.Instance.Image.ImageId != nil {
			awsRes.AMI = *env.Spec.Instance.Image.ImageId
		}
		for _, p := range env.Status.Properties {
			switch p.Name {
			case "InstanceId":
				awsRes.InstanceID = p.Value
			case "PublicDnsName":
				awsRes.PublicDNS = p.Value
			case "PublicIpAddress":
				awsRes.PublicIP = p.Value
			case "PrivateIpAddress":
				awsRes.PrivateIP = p.Value
			case "VpcId":
				awsRes.VpcID = p.Value
			case "SubnetId":
				awsRes.SubnetID = p.Value
			case "SecurityGroupId":
				awsRes.SecurityGroup = p.Value
			}
		}
		output.AWSResources = awsRes
	}

	return output
}

func (m command) printTableFormat(d *DescribeOutput) error {
	// Instance Information
	fmt.Println("=== Instance Information ===")
	fmt.Printf("ID:           %s\n", d.Instance.ID)
	fmt.Printf("Name:         %s\n", d.Instance.Name)
	fmt.Printf("Created:      %s (%s ago)\n", d.Instance.CreatedAt.Format("2006-01-02 15:04:05"), d.Instance.Age)
	fmt.Printf("Provisioned:  %v\n", d.Instance.Provisioned)
	fmt.Printf("Cache File:   %s\n", d.Instance.CacheFile)

	// Provider Information
	fmt.Println("\n=== Provider ===")
	fmt.Printf("Type:     %s\n", d.Provider.Type)
	if d.Provider.Region != "" {
		fmt.Printf("Region:   %s\n", d.Provider.Region)
	}
	fmt.Printf("Username: %s\n", d.Provider.Username)
	fmt.Printf("Key Name: %s\n", d.Provider.KeyName)

	// Cluster Information
	if d.Cluster != nil {
		fmt.Println("\n=== Cluster Configuration ===")
		fmt.Printf("Region:               %s\n", d.Cluster.Region)
		fmt.Printf("Control Plane Count:  %d\n", d.Cluster.ControlPlane.Count)
		fmt.Printf("Control Plane Type:   %s\n", d.Cluster.ControlPlane.InstanceType)
		if d.Cluster.ControlPlane.Dedicated {
			fmt.Printf("Control Plane Mode:   Dedicated (NoSchedule)\n")
		} else {
			fmt.Printf("Control Plane Mode:   Shared\n")
		}

		if d.Cluster.Workers != nil {
			fmt.Printf("Worker Count:         %d\n", d.Cluster.Workers.Count)
			fmt.Printf("Worker Type:          %s\n", d.Cluster.Workers.InstanceType)
		}

		if d.Cluster.HighAvailability != nil && d.Cluster.HighAvailability.Enabled {
			fmt.Printf("High Availability:    Enabled\n")
			fmt.Printf("Etcd Topology:        %s\n", d.Cluster.HighAvailability.EtcdTopology)
		}

		if d.Cluster.Phase != "" {
			fmt.Println("\n=== Cluster Status ===")
			fmt.Printf("Phase:       %s\n", d.Cluster.Phase)
			fmt.Printf("Total Nodes: %d\n", d.Cluster.TotalNodes)
			fmt.Printf("Ready Nodes: %d\n", d.Cluster.ReadyNodes)
			if d.Cluster.Endpoint != "" {
				fmt.Printf("Endpoint:    %s\n", d.Cluster.Endpoint)
			}
			if d.Cluster.LoadBalancerDNS != "" {
				fmt.Printf("LB DNS:      %s\n", d.Cluster.LoadBalancerDNS)
			}
		}

		if len(d.Cluster.Nodes) > 0 {
			fmt.Println("\n=== Nodes ===")
			for _, node := range d.Cluster.Nodes {
				fmt.Printf("  %s (%s)\n", node.Name, node.Role)
				fmt.Printf("    Instance ID: %s\n", node.InstanceID)
				fmt.Printf("    Public IP:   %s\n", node.PublicIP)
				fmt.Printf("    Private IP:  %s\n", node.PrivateIP)
				fmt.Printf("    Phase:       %s\n", node.Phase)
			}
		}
	}

	// Components
	fmt.Println("\n=== Components ===")
	if d.Components.Kernel != nil {
		fmt.Printf("Kernel:              %s\n", d.Components.Kernel.Version)
	}
	if d.Components.NVIDIADriver != nil {
		version := d.Components.NVIDIADriver.Version
		if version == "" {
			version = d.Components.NVIDIADriver.Branch
		}
		if version == "" {
			version = "latest"
		}
		fmt.Printf("NVIDIA Driver:       %s\n", version)
	}
	if d.Components.ContainerRuntime != nil {
		version := d.Components.ContainerRuntime.Version
		if version == "" {
			version = "latest"
		}
		fmt.Printf("Container Runtime:   %s (%s)\n", d.Components.ContainerRuntime.Name, version)
	}
	if d.Components.ContainerToolkit != nil {
		version := d.Components.ContainerToolkit.Version
		if version == "" {
			version = string(d.Components.ContainerToolkit.Source)
		}
		if version == "" {
			version = "latest"
		}
		cdi := ""
		if d.Components.ContainerToolkit.EnableCDI {
			cdi = " (CDI enabled)"
		}
		fmt.Printf("Container Toolkit:   %s%s\n", version, cdi)
	}
	if d.Components.Kubernetes != nil {
		version := d.Components.Kubernetes.Version
		if version == "" {
			version = "latest"
		}
		fmt.Printf("Kubernetes:          %s (%s)\n", version, d.Components.Kubernetes.Installer)
	}

	// AWS Resources
	if d.AWSResources != nil {
		fmt.Println("\n=== AWS Resources ===")
		if d.AWSResources.InstanceID != "" {
			fmt.Printf("Instance ID:     %s\n", d.AWSResources.InstanceID)
		}
		if d.AWSResources.InstanceType != "" {
			fmt.Printf("Instance Type:   %s\n", d.AWSResources.InstanceType)
		}
		if d.AWSResources.AMI != "" {
			fmt.Printf("AMI:             %s\n", d.AWSResources.AMI)
		}
		if d.AWSResources.PublicDNS != "" {
			fmt.Printf("Public DNS:      %s\n", d.AWSResources.PublicDNS)
		}
		if d.AWSResources.PublicIP != "" {
			fmt.Printf("Public IP:       %s\n", d.AWSResources.PublicIP)
		}
		if d.AWSResources.PrivateIP != "" {
			fmt.Printf("Private IP:      %s\n", d.AWSResources.PrivateIP)
		}
		if d.AWSResources.VpcID != "" {
			fmt.Printf("VPC ID:          %s\n", d.AWSResources.VpcID)
		}
		if d.AWSResources.SubnetID != "" {
			fmt.Printf("Subnet ID:       %s\n", d.AWSResources.SubnetID)
		}
		if d.AWSResources.SecurityGroup != "" {
			fmt.Printf("Security Group:  %s\n", d.AWSResources.SecurityGroup)
		}
	}

	// Status and Conditions
	fmt.Println("\n=== Status ===")
	fmt.Printf("State: %s\n", d.Status.State)
	if len(d.Status.Conditions) > 0 {
		fmt.Println("Conditions:")
		for _, cond := range d.Status.Conditions {
			fmt.Printf("  - %s: %s\n", cond.Type, cond.Status)
			if cond.Reason != "" {
				fmt.Printf("    Reason: %s\n", cond.Reason)
			}
			if cond.Message != "" {
				fmt.Printf("    Message: %s\n", cond.Message)
			}
		}
	}

	return nil
}
