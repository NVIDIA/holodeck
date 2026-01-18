/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package status

import (
	"fmt"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
	live      bool
}

// NewCommand constructs the status command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	// Create the 'status' command
	status := cli.Command{
		Name:  "status",
		Usage: "Show detailed information about a Holodeck instance",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.BoolFlag{
				Name:        "live",
				Aliases:     []string{"l"},
				Usage:       "Query live cluster status (requires SSH access)",
				Destination: &m.live,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("instance ID is required")
			}
			return m.run(c, c.Args().Get(0))
		},
	}

	return &status
}

func (m command) run(c *cli.Context, instanceID string) error {
	manager := instances.NewManager(m.log, m.cachePath)
	instance, err := manager.GetInstance(instanceID)
	if err != nil {
		// Check if this is an old cache file
		if instanceID == "" {
			return fmt.Errorf("invalid instance ID")
		}
		// Try to get the instance by filename (for old cache files)
		instance, err = manager.GetInstanceByFilename(instanceID)
		if err != nil {
			return fmt.Errorf("failed to get instance: %v", err)
		}
	}

	age := time.Since(instance.CreatedAt).Round(time.Second)

	fmt.Printf("Instance ID: %s\n", instance.ID)
	fmt.Printf("Name: %s\n", instance.Name)
	fmt.Printf("Provider: %s\n", instance.Provider)
	fmt.Printf("Status: %s\n", instance.Status)
	fmt.Printf("Created: %s (%s ago)\n",
		instance.CreatedAt.Format("2006-01-02 15:04:05"),
		age,
	)
	fmt.Printf("Cache File: %s\n", instance.CacheFile)

	// Check if this is a multinode cluster
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](instance.CacheFile)
	if err != nil {
		return nil // Just return basic info if we can't read the environment
	}

	// Display cluster information if available
	if env.Spec.Cluster != nil {
		fmt.Printf("\n--- Cluster Configuration ---\n")
		fmt.Printf("Region: %s\n", env.Spec.Cluster.Region)
		fmt.Printf("Control Plane Count: %d\n", env.Spec.Cluster.ControlPlane.Count)
		fmt.Printf("Control Plane Type: %s\n", env.Spec.Cluster.ControlPlane.InstanceType)
		if env.Spec.Cluster.ControlPlane.Dedicated {
			fmt.Printf("Control Plane Mode: Dedicated (NoSchedule)\n")
		} else {
			fmt.Printf("Control Plane Mode: Shared (workloads allowed)\n")
		}
		if env.Spec.Cluster.Workers != nil {
			fmt.Printf("Worker Count: %d\n", env.Spec.Cluster.Workers.Count)
			fmt.Printf("Worker Type: %s\n", env.Spec.Cluster.Workers.InstanceType)
		}
		if env.Spec.Cluster.HighAvailability != nil && env.Spec.Cluster.HighAvailability.Enabled {
			fmt.Printf("High Availability: Enabled\n")
			fmt.Printf("Etcd Topology: %s\n", env.Spec.Cluster.HighAvailability.EtcdTopology)
		}

		// Display node status from cache
		if env.Status.Cluster != nil {
			fmt.Printf("\n--- Cluster Status (cached) ---\n")
			fmt.Printf("Phase: %s\n", env.Status.Cluster.Phase)
			fmt.Printf("Total Nodes: %d\n", env.Status.Cluster.TotalNodes)
			fmt.Printf("Ready Nodes: %d\n", env.Status.Cluster.ReadyNodes)
			if env.Status.Cluster.ControlPlaneEndpoint != "" {
				fmt.Printf("Control Plane Endpoint: %s\n", env.Status.Cluster.ControlPlaneEndpoint)
			}
			if env.Status.Cluster.LoadBalancerDNS != "" {
				fmt.Printf("Load Balancer DNS: %s\n", env.Status.Cluster.LoadBalancerDNS)
			}

			if len(env.Status.Cluster.Nodes) > 0 {
				fmt.Printf("\nNodes:\n")
				for _, node := range env.Status.Cluster.Nodes {
					fmt.Printf("  - %s (%s)\n", node.Name, node.Role)
					fmt.Printf("    Instance ID: %s\n", node.InstanceID)
					fmt.Printf("    Public IP: %s\n", node.PublicIP)
					fmt.Printf("    Private IP: %s\n", node.PrivateIP)
					fmt.Printf("    Phase: %s\n", node.Phase)
				}
			}
		}

		// Get live cluster health if requested
		if m.live {
			fmt.Printf("\n--- Live Cluster Health ---\n")
			health, err := provisioner.GetClusterHealthFromEnv(m.log, &env)
			if err != nil {
				fmt.Printf("Unable to get live status: %v\n", err)
			} else {
				if health.Healthy {
					fmt.Printf("Status: Healthy\n")
				} else {
					fmt.Printf("Status: Degraded\n")
				}
				fmt.Printf("API Server: %s\n", health.APIServerStatus)
				fmt.Printf("Total Nodes: %d\n", health.TotalNodes)
				fmt.Printf("Ready Nodes: %d\n", health.ReadyNodes)
				fmt.Printf("Control Planes: %d\n", health.ControlPlanes)
				fmt.Printf("Workers: %d\n", health.Workers)
				fmt.Printf("Message: %s\n", health.Message)

				if len(health.Nodes) > 0 {
					fmt.Printf("\nLive Node Status:\n")
					for _, node := range health.Nodes {
						readyIcon := "✗"
						if node.Ready {
							readyIcon = "✓"
						}
						fmt.Printf("  %s %s (%s) - %s [%s]\n",
							readyIcon,
							node.Name,
							node.Role,
							node.Status,
							node.Version,
						)
					}
				}
			}
		}
	}

	return nil
}
