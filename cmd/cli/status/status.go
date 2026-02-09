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
	"strings"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/output"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log          *logger.FunLogger
	cachePath    string
	live         bool
	outputFormat string
}

// StatusOutput represents the instance status for JSON/YAML output
type StatusOutput struct {
	InstanceID string               `json:"instanceId" yaml:"instanceId"`
	Name       string               `json:"name" yaml:"name"`
	Provider   string               `json:"provider" yaml:"provider"`
	Status     string               `json:"status" yaml:"status"`
	CreatedAt  time.Time            `json:"createdAt" yaml:"createdAt"`
	Age        string               `json:"age" yaml:"age"`
	CacheFile  string               `json:"cacheFile" yaml:"cacheFile"`
	Cluster    *ClusterStatusOutput `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	LiveHealth *LiveHealthOutput    `json:"liveHealth,omitempty" yaml:"liveHealth,omitempty"`
}

// ClusterStatusOutput represents cluster configuration and status
type ClusterStatusOutput struct {
	Region               string             `json:"region" yaml:"region"`
	ControlPlaneCount    int32              `json:"controlPlaneCount" yaml:"controlPlaneCount"`
	ControlPlaneType     string             `json:"controlPlaneType" yaml:"controlPlaneType"`
	ControlPlaneMode     string             `json:"controlPlaneMode" yaml:"controlPlaneMode"`
	WorkerCount          int32              `json:"workerCount,omitempty" yaml:"workerCount,omitempty"`
	WorkerType           string             `json:"workerType,omitempty" yaml:"workerType,omitempty"`
	HighAvailability     *HAOutput          `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Phase                string             `json:"phase,omitempty" yaml:"phase,omitempty"`
	TotalNodes           int32              `json:"totalNodes,omitempty" yaml:"totalNodes,omitempty"`
	ReadyNodes           int32              `json:"readyNodes,omitempty" yaml:"readyNodes,omitempty"`
	ControlPlaneEndpoint string             `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	LoadBalancerDNS      string             `json:"loadBalancerDNS,omitempty" yaml:"loadBalancerDNS,omitempty"`
	Nodes                []NodeStatusOutput `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

// HAOutput represents high availability configuration
type HAOutput struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	EtcdTopology string `json:"etcdTopology,omitempty" yaml:"etcdTopology,omitempty"`
}

// NodeStatusOutput represents individual node status
type NodeStatusOutput struct {
	Name       string `json:"name" yaml:"name"`
	Role       string `json:"role" yaml:"role"`
	InstanceID string `json:"instanceId" yaml:"instanceId"`
	PublicIP   string `json:"publicIP" yaml:"publicIP"`
	PrivateIP  string `json:"privateIP" yaml:"privateIP"`
	Phase      string `json:"phase" yaml:"phase"`
}

// LiveHealthOutput represents live cluster health information
type LiveHealthOutput struct {
	Healthy         bool                   `json:"healthy" yaml:"healthy"`
	APIServerStatus string                 `json:"apiServerStatus" yaml:"apiServerStatus"`
	TotalNodes      int                    `json:"totalNodes" yaml:"totalNodes"`
	ReadyNodes      int                    `json:"readyNodes" yaml:"readyNodes"`
	ControlPlanes   int                    `json:"controlPlanes" yaml:"controlPlanes"`
	Workers         int                    `json:"workers" yaml:"workers"`
	Message         string                 `json:"message" yaml:"message"`
	Nodes           []LiveNodeStatusOutput `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

// LiveNodeStatusOutput represents live node status
type LiveNodeStatusOutput struct {
	Name    string `json:"name" yaml:"name"`
	Role    string `json:"role" yaml:"role"`
	Ready   bool   `json:"ready" yaml:"ready"`
	Status  string `json:"status" yaml:"status"`
	Version string `json:"version" yaml:"version"`
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
			return fmt.Errorf("failed to get instance: %w", err)
		}
	}

	age := time.Since(instance.CreatedAt).Round(time.Second)

	// Build output data structure
	statusOutput := &StatusOutput{
		InstanceID: instance.ID,
		Name:       instance.Name,
		Provider:   string(instance.Provider),
		Status:     instance.Status,
		CreatedAt:  instance.CreatedAt,
		Age:        age.String(),
		CacheFile:  instance.CacheFile,
	}

	// Check if this is a multinode cluster
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](instance.CacheFile)
	if err == nil && env.Spec.Cluster != nil {
		cpMode := "Shared (workloads allowed)"
		if env.Spec.Cluster.ControlPlane.Dedicated {
			cpMode = "Dedicated (NoSchedule)"
		}

		statusOutput.Cluster = &ClusterStatusOutput{
			Region:            env.Spec.Cluster.Region,
			ControlPlaneCount: env.Spec.Cluster.ControlPlane.Count,
			ControlPlaneType:  env.Spec.Cluster.ControlPlane.InstanceType,
			ControlPlaneMode:  cpMode,
		}

		if env.Spec.Cluster.Workers != nil {
			statusOutput.Cluster.WorkerCount = env.Spec.Cluster.Workers.Count
			statusOutput.Cluster.WorkerType = env.Spec.Cluster.Workers.InstanceType
		}

		if env.Spec.Cluster.HighAvailability != nil && env.Spec.Cluster.HighAvailability.Enabled {
			statusOutput.Cluster.HighAvailability = &HAOutput{
				Enabled:      true,
				EtcdTopology: string(env.Spec.Cluster.HighAvailability.EtcdTopology),
			}
		}

		// Add cluster status from cache
		if env.Status.Cluster != nil {
			statusOutput.Cluster.Phase = env.Status.Cluster.Phase
			statusOutput.Cluster.TotalNodes = env.Status.Cluster.TotalNodes
			statusOutput.Cluster.ReadyNodes = env.Status.Cluster.ReadyNodes
			statusOutput.Cluster.ControlPlaneEndpoint = env.Status.Cluster.ControlPlaneEndpoint
			statusOutput.Cluster.LoadBalancerDNS = env.Status.Cluster.LoadBalancerDNS

			if len(env.Status.Cluster.Nodes) > 0 {
				statusOutput.Cluster.Nodes = make([]NodeStatusOutput, 0, len(env.Status.Cluster.Nodes))
				for _, node := range env.Status.Cluster.Nodes {
					statusOutput.Cluster.Nodes = append(statusOutput.Cluster.Nodes, NodeStatusOutput{
						Name:       node.Name,
						Role:       node.Role,
						InstanceID: node.InstanceID,
						PublicIP:   node.PublicIP,
						PrivateIP:  node.PrivateIP,
						Phase:      node.Phase,
					})
				}
			}
		}

		// Get live cluster health if requested
		if m.live {
			health, err := provisioner.GetClusterHealthFromEnv(m.log, &env)
			if err == nil {
				statusOutput.LiveHealth = &LiveHealthOutput{
					Healthy:         health.Healthy,
					APIServerStatus: health.APIServerStatus,
					TotalNodes:      health.TotalNodes,
					ReadyNodes:      health.ReadyNodes,
					ControlPlanes:   health.ControlPlanes,
					Workers:         health.Workers,
					Message:         health.Message,
				}

				if len(health.Nodes) > 0 {
					statusOutput.LiveHealth.Nodes = make([]LiveNodeStatusOutput, 0, len(health.Nodes))
					for _, node := range health.Nodes {
						statusOutput.LiveHealth.Nodes = append(statusOutput.LiveHealth.Nodes, LiveNodeStatusOutput{
							Name:    node.Name,
							Role:    node.Role,
							Ready:   node.Ready,
							Status:  node.Status,
							Version: node.Version,
						})
					}
				}
			}
		}
	}

	// Create formatter and output
	formatter, err := output.NewFormatter(m.outputFormat)
	if err != nil {
		return fmt.Errorf("invalid output format %q, must be one of: %s", m.outputFormat, strings.Join(output.ValidFormats(), ", "))
	}

	// For table format, use custom formatting
	if formatter.Format() == output.FormatTable {
		return m.printTableFormat(statusOutput)
	}

	return formatter.Print(statusOutput)
}

// printTableFormat outputs status in the original human-readable format
//
//nolint:errcheck // stdout writes
func (m command) printTableFormat(s *StatusOutput) error {
	fmt.Printf("Instance ID: %s\n", s.InstanceID)
	fmt.Printf("Name: %s\n", s.Name)
	fmt.Printf("Provider: %s\n", s.Provider)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Printf("Created: %s (%s ago)\n", s.CreatedAt.Format("2006-01-02 15:04:05"), s.Age)
	fmt.Printf("Cache File: %s\n", s.CacheFile)

	if s.Cluster != nil {
		fmt.Printf("\n--- Cluster Configuration ---\n")
		fmt.Printf("Region: %s\n", s.Cluster.Region)
		fmt.Printf("Control Plane Count: %d\n", s.Cluster.ControlPlaneCount)
		fmt.Printf("Control Plane Type: %s\n", s.Cluster.ControlPlaneType)
		fmt.Printf("Control Plane Mode: %s\n", s.Cluster.ControlPlaneMode)

		if s.Cluster.WorkerCount > 0 {
			fmt.Printf("Worker Count: %d\n", s.Cluster.WorkerCount)
			fmt.Printf("Worker Type: %s\n", s.Cluster.WorkerType)
		}

		if s.Cluster.HighAvailability != nil && s.Cluster.HighAvailability.Enabled {
			fmt.Printf("High Availability: Enabled\n")
			fmt.Printf("Etcd Topology: %s\n", s.Cluster.HighAvailability.EtcdTopology)
		}

		if s.Cluster.Phase != "" {
			fmt.Printf("\n--- Cluster Status (cached) ---\n")
			fmt.Printf("Phase: %s\n", s.Cluster.Phase)
			fmt.Printf("Total Nodes: %d\n", s.Cluster.TotalNodes)
			fmt.Printf("Ready Nodes: %d\n", s.Cluster.ReadyNodes)
			if s.Cluster.ControlPlaneEndpoint != "" {
				fmt.Printf("Control Plane Endpoint: %s\n", s.Cluster.ControlPlaneEndpoint)
			}
			if s.Cluster.LoadBalancerDNS != "" {
				fmt.Printf("Load Balancer DNS: %s\n", s.Cluster.LoadBalancerDNS)
			}

			if len(s.Cluster.Nodes) > 0 {
				fmt.Printf("\nNodes:\n")
				for _, node := range s.Cluster.Nodes {
					fmt.Printf("  - %s (%s)\n", node.Name, node.Role)
					fmt.Printf("    Instance ID: %s\n", node.InstanceID)
					fmt.Printf("    Public IP: %s\n", node.PublicIP)
					fmt.Printf("    Private IP: %s\n", node.PrivateIP)
					fmt.Printf("    Phase: %s\n", node.Phase)
				}
			}
		}

		if s.LiveHealth != nil {
			fmt.Printf("\n--- Live Cluster Health ---\n")
			if s.LiveHealth.Healthy {
				fmt.Printf("Status: Healthy\n")
			} else {
				fmt.Printf("Status: Degraded\n")
			}
			fmt.Printf("API Server: %s\n", s.LiveHealth.APIServerStatus)
			fmt.Printf("Total Nodes: %d\n", s.LiveHealth.TotalNodes)
			fmt.Printf("Ready Nodes: %d\n", s.LiveHealth.ReadyNodes)
			fmt.Printf("Control Planes: %d\n", s.LiveHealth.ControlPlanes)
			fmt.Printf("Workers: %d\n", s.LiveHealth.Workers)
			fmt.Printf("Message: %s\n", s.LiveHealth.Message)

			if len(s.LiveHealth.Nodes) > 0 {
				fmt.Printf("\nLive Node Status:\n")
				for _, node := range s.LiveHealth.Nodes {
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

	return nil
}
