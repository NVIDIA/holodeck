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

package get

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/common"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/utils"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
	output    string
	node      string
}

// NewCommand constructs the get command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	// Create the 'get' command with subcommands
	getCmd := cli.Command{
		Name:  "get",
		Usage: "Get resources from a Holodeck instance",
		Subcommands: []*cli.Command{
			m.buildKubeconfigSubcommand(),
			m.buildSSHConfigSubcommand(),
		},
	}

	return &getCmd
}

func (m command) buildKubeconfigSubcommand() *cli.Command {
	return &cli.Command{
		Name:      "kubeconfig",
		Usage:     "Download kubeconfig from a Holodeck instance",
		ArgsUsage: "<instance-id>",
		Description: `Download the kubeconfig file from a Kubernetes-enabled Holodeck instance.

Examples:
  # Download kubeconfig to default location (~/.kube/config-<instance-id>)
  holodeck get kubeconfig abc123

  # Download kubeconfig to a specific file
  holodeck get kubeconfig abc123 -o ./my-kubeconfig

  # For multinode clusters, download from a specific control-plane
  holodeck get kubeconfig abc123 --node cp-0`,
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
				Usage:       "Output file path (default: ~/.kube/config-<instance-id>)",
				Destination: &m.output,
			},
			&cli.StringFlag{
				Name:        "node",
				Aliases:     []string{"n"},
				Usage:       "Node name for multinode clusters (default: first control-plane)",
				Destination: &m.node,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required")
			}
			return m.runKubeconfig(c.Args().Get(0))
		},
	}
}

func (m command) buildSSHConfigSubcommand() *cli.Command {
	return &cli.Command{
		Name:      "ssh-config",
		Usage:     "Generate SSH config entry for a Holodeck instance",
		ArgsUsage: "<instance-id>",
		Description: `Generate an SSH config entry that can be added to ~/.ssh/config.

Examples:
  # Print SSH config to stdout
  holodeck get ssh-config abc123

  # Append to SSH config file
  holodeck get ssh-config abc123 >> ~/.ssh/config`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.StringFlag{
				Name:        "node",
				Aliases:     []string{"n"},
				Usage:       "Node name for multinode clusters (default: all nodes)",
				Destination: &m.node,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required")
			}
			return m.runSSHConfig(c.Args().Get(0))
		},
	}
}

func (m command) runKubeconfig(instanceID string) error {
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

	// Check if Kubernetes is installed
	if !env.Spec.Kubernetes.Install {
		return fmt.Errorf("instance %s does not have Kubernetes installed", instanceID)
	}

	// Determine host URL
	hostUrl, err := common.GetHostURL(&env, m.node, true)
	if err != nil {
		return fmt.Errorf("failed to get host URL: %v", err)
	}

	// Determine output path
	outputPath := m.output
	if outputPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}
		kubeDir := filepath.Join(homeDir, ".kube")
		if err := os.MkdirAll(kubeDir, 0755); err != nil {
			return fmt.Errorf("failed to create .kube directory: %v", err)
		}
		outputPath = filepath.Join(kubeDir, fmt.Sprintf("config-%s", instanceID))
	}

	// Download kubeconfig
	if err := utils.GetKubeConfig(m.log, &env, hostUrl, outputPath); err != nil {
		return fmt.Errorf("failed to download kubeconfig: %v", err)
	}

	m.log.Info("Kubeconfig saved to: %s", outputPath)
	m.log.Info("To use: export KUBECONFIG=%s", outputPath)

	return nil
}

func (m command) runSSHConfig(instanceID string) error {
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

	userName := env.Spec.Username
	if userName == "" {
		userName = "ubuntu"
	}
	keyPath := env.Spec.PrivateKey

	// For multinode clusters, generate config for all or specific nodes
	if env.Spec.Cluster != nil && env.Status.Cluster != nil && len(env.Status.Cluster.Nodes) > 0 {
		return m.generateClusterSSHConfig(instanceID, &env, userName, keyPath)
	}

	// Single node
	hostUrl, err := common.GetHostURL(&env, m.node, false)
	if err != nil {
		return fmt.Errorf("failed to get host URL: %v", err)
	}

	fmt.Printf("# Holodeck instance: %s\n", instanceID)
	fmt.Printf("Host holodeck-%s\n", instanceID)
	fmt.Printf("    HostName %s\n", hostUrl)
	fmt.Printf("    User %s\n", userName)
	fmt.Printf("    IdentityFile %s\n", keyPath)
	fmt.Printf("    StrictHostKeyChecking no\n")
	fmt.Printf("    UserKnownHostsFile /dev/null\n")
	fmt.Printf("\n")

	return nil
}

func (m command) generateClusterSSHConfig(instanceID string, env *v1alpha1.Environment, userName, keyPath string) error {
	fmt.Printf("# Holodeck cluster: %s\n", instanceID)

	for _, node := range env.Status.Cluster.Nodes {
		// If a specific node was requested, only show that one
		if m.node != "" && node.Name != m.node {
			continue
		}

		fmt.Printf("Host holodeck-%s-%s\n", instanceID, node.Name)
		fmt.Printf("    HostName %s\n", node.PublicIP)
		fmt.Printf("    User %s\n", userName)
		fmt.Printf("    IdentityFile %s\n", keyPath)
		fmt.Printf("    StrictHostKeyChecking no\n")
		fmt.Printf("    UserKnownHostsFile /dev/null\n")
		fmt.Printf("\n")
	}

	return nil
}

