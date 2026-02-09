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

package list

import (
	"fmt"
	"strings"
	"time"

	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/output"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log          *logger.FunLogger
	cachePath    string
	idsOnly      bool
	outputFormat string
}

// InstanceList represents a list of instances for output formatting
type InstanceList struct {
	Instances []InstanceOutput `json:"instances" yaml:"instances"`
}

// InstanceOutput represents instance data for JSON/YAML output
type InstanceOutput struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Provider    string       `json:"provider" yaml:"provider"`
	Type        string       `json:"type" yaml:"type"`
	Nodes       string       `json:"nodes" yaml:"nodes"`
	Status      string       `json:"status" yaml:"status"`
	Provisioned bool         `json:"provisioned" yaml:"provisioned"`
	Age         string       `json:"age" yaml:"age"`
	CreatedAt   time.Time    `json:"createdAt" yaml:"createdAt"`
	Cluster     *ClusterInfo `json:"cluster,omitempty" yaml:"cluster,omitempty"`
}

// ClusterInfo represents cluster-specific information
type ClusterInfo struct {
	Region               string `json:"region,omitempty" yaml:"region,omitempty"`
	ControlPlaneCount    int32  `json:"controlPlaneCount,omitempty" yaml:"controlPlaneCount,omitempty"`
	WorkerCount          int32  `json:"workerCount,omitempty" yaml:"workerCount,omitempty"`
	TotalNodes           int32  `json:"totalNodes,omitempty" yaml:"totalNodes,omitempty"`
	ReadyNodes           int32  `json:"readyNodes,omitempty" yaml:"readyNodes,omitempty"`
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	HAEnabled            bool   `json:"haEnabled,omitempty" yaml:"haEnabled,omitempty"`
}

// Headers implements output.TableData
func (l *InstanceList) Headers() []string {
	return []string{"INSTANCE ID", "NAME", "PROVIDER", "TYPE", "NODES", "STATUS", "PROVISIONED", "AGE"}
}

// Rows implements output.TableData
func (l *InstanceList) Rows() [][]string {
	rows := make([][]string, 0, len(l.Instances))
	for _, inst := range l.Instances {
		rows = append(rows, []string{
			inst.ID,
			inst.Name,
			inst.Provider,
			inst.Type,
			inst.Nodes,
			inst.Status,
			fmt.Sprintf("%v", inst.Provisioned),
			inst.Age,
		})
	}
	return rows
}

// NewCommand constructs the list command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := &command{
		log: log,
	}
	return c.build()
}

func (m *command) build() *cli.Command {
	// Create the 'list' command
	list := cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List all Holodeck instances",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.BoolFlag{
				Name:        "ids-only",
				Aliases:     []string{"q"},
				Usage:       "Only display instance IDs (short: -q)",
				Destination: &m.idsOnly,
			},
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "Output format: table, json, yaml (default: table)",
				Destination: &m.outputFormat,
				Value:       "table",
			},
		},
		Action: m.run,
	}

	return &list
}

func (m *command) run(c *cli.Context) error {
	manager := instances.NewManager(m.log, m.cachePath)
	instList, err := manager.ListInstances()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instList) == 0 {
		m.log.Info("No instances found")
		return nil
	}

	// If ids-only mode is enabled, only print instance IDs
	if m.idsOnly {
		for _, instance := range instList {
			if instance.ID == "" {
				continue
			}
			fmt.Println(instance.ID)
		}
		return nil
	}

	// Build output data
	outputData := &InstanceList{
		Instances: make([]InstanceOutput, 0, len(instList)),
	}

	for _, instance := range instList {
		// Skip instances without an ID (old cache files)
		if instance.ID == "" {
			continue
		}

		age := time.Since(instance.CreatedAt).Round(time.Second)
		instanceType := "single"
		nodes := "1"
		var clusterInfo *ClusterInfo

		if instance.IsCluster && instance.ClusterInfo != nil {
			if instance.ClusterInfo.HAEnabled {
				instanceType = "cluster-ha"
			} else {
				instanceType = "cluster"
			}
			nodes = fmt.Sprintf("%d/%d",
				instance.ClusterInfo.ReadyNodes,
				instance.ClusterInfo.TotalNodes,
			)
			clusterInfo = &ClusterInfo{
				Region:               instance.ClusterInfo.Region,
				ControlPlaneCount:    instance.ClusterInfo.ControlPlaneCount,
				WorkerCount:          instance.ClusterInfo.WorkerCount,
				TotalNodes:           instance.ClusterInfo.TotalNodes,
				ReadyNodes:           instance.ClusterInfo.ReadyNodes,
				ControlPlaneEndpoint: instance.ClusterInfo.ControlPlaneEndpoint,
				HAEnabled:            instance.ClusterInfo.HAEnabled,
			}
		}

		outputData.Instances = append(outputData.Instances, InstanceOutput{
			ID:          instance.ID,
			Name:        instance.Name,
			Provider:    string(instance.Provider),
			Type:        instanceType,
			Nodes:       nodes,
			Status:      instance.Status,
			Provisioned: instance.Provisioned,
			Age:         age.String(),
			CreatedAt:   instance.CreatedAt,
			Cluster:     clusterInfo,
		})
	}

	// Create formatter and output
	formatter, err := output.NewFormatter(m.outputFormat)
	if err != nil {
		return fmt.Errorf("invalid output format %q, must be one of: %s", m.outputFormat, strings.Join(output.ValidFormats(), ", "))
	}

	return formatter.Print(outputData)
}
