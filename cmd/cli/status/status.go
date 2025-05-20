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

	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
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

	return nil
}
