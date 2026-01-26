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

package provision

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/pkg/utils"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log        *logger.FunLogger
	cachePath  string
	kubeconfig string

	// SSH provider flags
	sshMode  bool
	host     string
	keyPath  string
	username string
	envFile  string
}

// NewCommand constructs the provision command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m *command) build() *cli.Command {
	provisionCmd := cli.Command{
		Name:      "provision",
		Usage:     "Provision or re-provision a Holodeck instance",
		ArgsUsage: "[instance-id]",
		Description: `Provision or re-provision an existing Holodeck instance.

This command runs the provisioning scripts on an instance. Because templates
are idempotent, it's safe to re-run provisioning to add components or recover
from failures.

Modes:
  1. Instance mode: Provision an existing instance by ID
  2. SSH mode: Provision a remote host directly (no instance required)

Examples:
  # Provision an existing instance
  holodeck provision abc123

  # Re-provision with kubeconfig download
  holodeck provision abc123 -k ./kubeconfig

  # SSH mode: Provision a remote host directly
  holodeck provision --ssh --host 1.2.3.4 --key ~/.ssh/id_rsa -f env.yaml

  # SSH mode with custom username
  holodeck provision --ssh --host myhost.example.com --key ~/.ssh/key --user ec2-user -f env.yaml`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.StringFlag{
				Name:        "kubeconfig",
				Aliases:     []string{"k"},
				Usage:       "Path to save the kubeconfig file",
				Destination: &m.kubeconfig,
			},
			// SSH mode flags
			&cli.BoolFlag{
				Name:        "ssh",
				Usage:       "SSH mode: provision a remote host directly",
				Destination: &m.sshMode,
			},
			&cli.StringFlag{
				Name:        "host",
				Usage:       "SSH mode: remote host address",
				Destination: &m.host,
			},
			&cli.StringFlag{
				Name:        "key",
				Usage:       "SSH mode: path to SSH private key",
				Destination: &m.keyPath,
			},
			&cli.StringFlag{
				Name:        "user",
				Aliases:     []string{"u"},
				Usage:       "SSH mode: SSH username (default: ubuntu)",
				Destination: &m.username,
				Value:       "ubuntu",
			},
			&cli.StringFlag{
				Name:        "envFile",
				Aliases:     []string{"f"},
				Usage:       "Path to the Environment file (required for SSH mode)",
				Destination: &m.envFile,
			},
		},
		Action: func(c *cli.Context) error {
			if m.sshMode {
				return m.runSSHMode()
			}

			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required (or use --ssh mode)")
			}
			return m.runInstanceMode(c.Args().Get(0))
		},
	}

	return &provisionCmd
}

func (m *command) runInstanceMode(instanceID string) error {
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

	m.log.Info("Provisioning instance %s...", instanceID)

	// Run provisioning based on instance type
	if env.Spec.Cluster != nil && env.Status.Cluster != nil && len(env.Status.Cluster.Nodes) > 0 {
		if err := m.runClusterProvision(&env); err != nil {
			return err
		}
	} else {
		if err := m.runSingleNodeProvision(&env); err != nil {
			return err
		}
	}

	// Update provisioned status
	env.Labels[instances.InstanceProvisionedLabelKey] = "true"
	data, err := jyaml.MarshalYAML(env)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %v", err)
	}
	if err := os.WriteFile(instance.CacheFile, data, 0600); err != nil {
		return fmt.Errorf("failed to update cache file: %v", err)
	}

	// Download kubeconfig if requested and Kubernetes is installed
	if m.kubeconfig != "" && env.Spec.Kubernetes.Install {
		hostUrl, err := m.getHostURL(&env)
		if err != nil {
			m.log.Warning("Failed to get host URL for kubeconfig: %v", err)
		} else {
			if err := utils.GetKubeConfig(m.log, &env, hostUrl, m.kubeconfig); err != nil {
				m.log.Warning("Failed to download kubeconfig: %v", err)
			}
		}
	}

	m.log.Info("✅ Provisioning completed successfully")
	return nil
}

func (m *command) runSSHMode() error {
	// Validate SSH mode flags
	if m.host == "" {
		return fmt.Errorf("--host is required in SSH mode")
	}
	if m.keyPath == "" {
		return fmt.Errorf("--key is required in SSH mode")
	}
	if m.envFile == "" {
		return fmt.Errorf("--envFile/-f is required in SSH mode")
	}

	// Load environment file
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](m.envFile)
	if err != nil {
		return fmt.Errorf("failed to read environment file: %v", err)
	}

	// Override with SSH mode settings
	env.Spec.Provider = v1alpha1.ProviderSSH
	env.Spec.HostUrl = m.host
	env.Spec.PrivateKey = m.keyPath
	env.Spec.Username = m.username

	m.log.Info("Provisioning %s via SSH...", m.host)

	// Create provisioner and run
	p, err := provisioner.New(m.log, m.keyPath, m.username, m.host)
	if err != nil {
		return fmt.Errorf("failed to create provisioner: %v", err)
	}
	defer p.Client.Close()

	if err := p.Run(env); err != nil {
		return fmt.Errorf("provisioning failed: %v", err)
	}

	// Download kubeconfig if requested and Kubernetes is installed
	if m.kubeconfig != "" && env.Spec.Kubernetes.Install {
		if err := utils.GetKubeConfig(m.log, &env, m.host, m.kubeconfig); err != nil {
			m.log.Warning("Failed to download kubeconfig: %v", err)
		}
	}

	m.log.Info("✅ Provisioning completed successfully")
	return nil
}

func (m *command) runSingleNodeProvision(env *v1alpha1.Environment) error {
	hostUrl, err := m.getHostURL(env)
	if err != nil {
		return fmt.Errorf("failed to get host URL: %v", err)
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

func (m *command) getHostURL(env *v1alpha1.Environment) (string, error) {
	// For multinode clusters, get first control-plane
	if env.Spec.Cluster != nil && env.Status.Cluster != nil && len(env.Status.Cluster.Nodes) > 0 {
		for _, node := range env.Status.Cluster.Nodes {
			if node.Role == "control-plane" {
				return node.PublicIP, nil
			}
		}
		return env.Status.Cluster.Nodes[0].PublicIP, nil
	}

	// Single node - get from properties
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range env.Status.Properties {
			if p.Name == aws.PublicDnsName {
				return p.Value, nil
			}
		}
	} else if env.Spec.Provider == v1alpha1.ProviderSSH {
		return env.Spec.HostUrl, nil
	}

	return "", fmt.Errorf("unable to determine host URL")
}

// getKubeconfigPath returns the path to save kubeconfig
func getKubeconfigPath(instanceID string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("kubeconfig-%s", instanceID)
	}
	kubeDir := filepath.Join(homeDir, ".kube")
	_ = os.MkdirAll(kubeDir, 0755)
	return filepath.Join(kubeDir, fmt.Sprintf("config-%s", instanceID))
}
