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

package update

import (
	"fmt"
	"os"
	"strings"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string

	// Component flags
	addDriver     bool
	driverVersion string
	driverBranch  string

	addRuntime    bool
	runtimeName   string
	runtimeVer    string

	addToolkit    bool
	toolkitVer    string
	enableCDI     bool

	addKubernetes bool
	k8sInstaller  string
	k8sVersion    string

	// Label flags
	labels cli.StringSlice

	// Reprovision flag
	reprovision bool
}

// NewCommand constructs the update command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m *command) build() *cli.Command {
	updateCmd := cli.Command{
		Name:      "update",
		Usage:     "Update a Holodeck instance configuration",
		ArgsUsage: "<instance-id>",
		Description: `Update an existing Holodeck instance by adding components or modifying configuration.

Because provisioning templates are idempotent, components that are already installed
will be skipped, making it safe to add new components to existing instances.

Examples:
  # Add NVIDIA driver to an existing instance
  holodeck update abc123 --add-driver

  # Add driver with specific version
  holodeck update abc123 --add-driver --driver-version 560.35.03

  # Add Kubernetes to an instance
  holodeck update abc123 --add-kubernetes --k8s-version v1.31.1

  # Add container toolkit with CDI enabled
  holodeck update abc123 --add-toolkit --enable-cdi

  # Add labels to an instance
  holodeck update abc123 --label team=gpu-infra --label env=test

  # Re-provision an instance (re-run all provisioning)
  holodeck update abc123 --reprovision`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			// Driver flags
			&cli.BoolFlag{
				Name:        "add-driver",
				Usage:       "Add NVIDIA driver to the instance",
				Destination: &m.addDriver,
			},
			&cli.StringFlag{
				Name:        "driver-version",
				Usage:       "NVIDIA driver version",
				Destination: &m.driverVersion,
			},
			&cli.StringFlag{
				Name:        "driver-branch",
				Usage:       "NVIDIA driver branch (e.g., 560, 550)",
				Destination: &m.driverBranch,
			},
			// Runtime flags
			&cli.BoolFlag{
				Name:        "add-runtime",
				Usage:       "Add container runtime to the instance",
				Destination: &m.addRuntime,
			},
			&cli.StringFlag{
				Name:        "runtime-name",
				Usage:       "Container runtime name (containerd, docker, crio)",
				Destination: &m.runtimeName,
				Value:       "containerd",
			},
			&cli.StringFlag{
				Name:        "runtime-version",
				Usage:       "Container runtime version",
				Destination: &m.runtimeVer,
			},
			// Toolkit flags
			&cli.BoolFlag{
				Name:        "add-toolkit",
				Usage:       "Add NVIDIA Container Toolkit to the instance",
				Destination: &m.addToolkit,
			},
			&cli.StringFlag{
				Name:        "toolkit-version",
				Usage:       "NVIDIA Container Toolkit version",
				Destination: &m.toolkitVer,
			},
			&cli.BoolFlag{
				Name:        "enable-cdi",
				Usage:       "Enable CDI (Container Device Interface)",
				Destination: &m.enableCDI,
			},
			// Kubernetes flags
			&cli.BoolFlag{
				Name:        "add-kubernetes",
				Usage:       "Add Kubernetes to the instance",
				Destination: &m.addKubernetes,
			},
			&cli.StringFlag{
				Name:        "k8s-installer",
				Usage:       "Kubernetes installer (kubeadm, kind, microk8s)",
				Destination: &m.k8sInstaller,
				Value:       "kubeadm",
			},
			&cli.StringFlag{
				Name:        "k8s-version",
				Usage:       "Kubernetes version",
				Destination: &m.k8sVersion,
			},
			// Labels
			&cli.StringSliceFlag{
				Name:        "label",
				Aliases:     []string{"l"},
				Usage:       "Add label (key=value format, can be repeated)",
				Destination: &m.labels,
			},
			// Reprovision
			&cli.BoolFlag{
				Name:        "reprovision",
				Usage:       "Re-run provisioning on the instance",
				Destination: &m.reprovision,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required")
			}
			return m.run(c.Args().Get(0))
		},
	}

	return &updateCmd
}

func (m *command) run(instanceID string) error {
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

	// Track if we need to reprovision
	needsProvision := m.reprovision
	configChanged := false

	// Apply component updates
	if m.addDriver {
		if !env.Spec.NVIDIADriver.Install {
			env.Spec.NVIDIADriver.Install = true
			configChanged = true
			needsProvision = true
		}
		if m.driverVersion != "" {
			env.Spec.NVIDIADriver.Version = m.driverVersion
			configChanged = true
		}
		if m.driverBranch != "" {
			env.Spec.NVIDIADriver.Branch = m.driverBranch
			configChanged = true
		}
	}

	if m.addRuntime {
		if !env.Spec.ContainerRuntime.Install {
			env.Spec.ContainerRuntime.Install = true
			configChanged = true
			needsProvision = true
		}
		if m.runtimeName != "" {
			env.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeName(m.runtimeName)
			configChanged = true
		}
		if m.runtimeVer != "" {
			env.Spec.ContainerRuntime.Version = m.runtimeVer
			configChanged = true
		}
	}

	if m.addToolkit {
		if !env.Spec.NVIDIAContainerToolkit.Install {
			env.Spec.NVIDIAContainerToolkit.Install = true
			configChanged = true
			needsProvision = true
		}
		if m.toolkitVer != "" {
			env.Spec.NVIDIAContainerToolkit.Version = m.toolkitVer
			configChanged = true
		}
		if m.enableCDI {
			env.Spec.NVIDIAContainerToolkit.EnableCDI = true
			configChanged = true
			needsProvision = true
		}
	}

	if m.addKubernetes {
		if !env.Spec.Kubernetes.Install {
			env.Spec.Kubernetes.Install = true
			configChanged = true
			needsProvision = true
		}
		if m.k8sInstaller != "" {
			env.Spec.Kubernetes.KubernetesInstaller = m.k8sInstaller
			configChanged = true
		}
		if m.k8sVersion != "" {
			env.Spec.Kubernetes.KubernetesVersion = m.k8sVersion
			if env.Spec.Kubernetes.Release == nil {
				env.Spec.Kubernetes.Release = &v1alpha1.K8sReleaseSpec{}
			}
			env.Spec.Kubernetes.Release.Version = m.k8sVersion
			configChanged = true
		}
	}

	// Apply labels
	labels := m.labels.Value()
	if len(labels) > 0 {
		if env.Labels == nil {
			env.Labels = make(map[string]string)
		}
		for _, label := range labels {
			parts := strings.SplitN(label, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label format: %s (expected key=value)", label)
			}
			env.Labels[parts[0]] = parts[1]
			configChanged = true
		}

		// Update AWS resource tags if applicable
		if env.Spec.Provider == v1alpha1.ProviderAWS {
			if err := m.updateAWSTags(&env, labels); err != nil {
				m.log.Warning("Failed to update AWS tags: %v", err)
			}
		}
	}

	// Save updated config if changed
	if configChanged {
		data, err := jyaml.MarshalYAML(env)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %v", err)
		}
		if err := os.WriteFile(instance.CacheFile, data, 0600); err != nil {
			return fmt.Errorf("failed to update cache file: %v", err)
		}
		m.log.Info("Configuration updated")
	}

	// Run provisioning if needed
	if needsProvision {
		m.log.Info("Running provisioning...")
		if err := m.runProvision(&env); err != nil {
			return fmt.Errorf("provisioning failed: %v", err)
		}

		// Mark as provisioned
		env.Labels[instances.InstanceProvisionedLabelKey] = "true"
		data, err := jyaml.MarshalYAML(env)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %v", err)
		}
		if err := os.WriteFile(instance.CacheFile, data, 0600); err != nil {
			return fmt.Errorf("failed to update cache file: %v", err)
		}

		m.log.Info("Provisioning completed successfully")
	}

	if !configChanged && !needsProvision {
		m.log.Info("No changes to apply")
	}

	return nil
}

func (m *command) runProvision(env *v1alpha1.Environment) error {
	// Determine host URL
	var hostUrl string

	if env.Spec.Cluster != nil && env.Status.Cluster != nil && len(env.Status.Cluster.Nodes) > 0 {
		// For clusters, provision all nodes
		return m.runClusterProvision(env)
	}

	// Single node
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range env.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
	} else if env.Spec.Provider == v1alpha1.ProviderSSH {
		hostUrl = env.Spec.HostUrl
	}

	if hostUrl == "" {
		return fmt.Errorf("unable to determine host URL")
	}

	// Create provisioner and run
	p, err := provisioner.New(m.log, env.Spec.PrivateKey, env.Spec.Username, hostUrl)
	if err != nil {
		return fmt.Errorf("failed to create provisioner: %v", err)
	}
	defer p.Client.Close()

	return p.Run(*env)
}

func (m *command) runClusterProvision(env *v1alpha1.Environment) error {
	// Build node list from cluster status
	var nodes []provisioner.NodeInfo
	for _, node := range env.Status.Cluster.Nodes {
		nodes = append(nodes, provisioner.NodeInfo{
			Name:        node.Name,
			PublicIP:    node.PublicIP,
			PrivateIP:   node.PrivateIP,
			Role:        node.Role,
			SSHUsername: node.SSHUsername,
		})
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster status")
	}

	// Create cluster provisioner
	cp := provisioner.NewClusterProvisioner(
		m.log,
		env.Spec.PrivateKey,
		env.Spec.Username,
		env,
	)

	return cp.ProvisionCluster(nodes)
}

func (m *command) updateAWSTags(env *v1alpha1.Environment, labels []string) error {
	// Convert labels to tags map
	tags := make(map[string]string)
	for _, label := range labels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) == 2 {
			tags[parts[0]] = parts[1]
		}
	}

	// Create AWS provider and update tags
	client, err := aws.New(m.log, *env, "")
	if err != nil {
		return err
	}

	// Get instance ID from properties
	var instanceID string
	for _, p := range env.Status.Properties {
		if p.Name == "InstanceId" {
			instanceID = p.Value
			break
		}
	}

	if instanceID == "" {
		return fmt.Errorf("instance ID not found in status")
	}

	return client.UpdateResourcesTags(tags, instanceID)
}
