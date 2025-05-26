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

package delete

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
}

// NewCommand constructs the delete command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	// Create the 'delete' command
	delete := cli.Command{
		Name:  "delete",
		Usage: "Delete a Holodeck instance",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
				Value:       filepath.Join(os.Getenv("HOME"), ".cache", "holodeck"),
			},
			&cli.StringFlag{
				Name:    "instance-id",
				Aliases: []string{"i"},
				Usage:   "Instance ID to delete",
			},
		},

		Action: func(c *cli.Context) error {
			// Delete using instance ID
			instanceID := c.String("instance-id")
			return m.run(c, instanceID)
		},
	}

	return &delete
}

func (m command) run(c *cli.Context, instanceID string) error {
	manager := instances.NewManager(m.log, m.cachePath)

	// First check if the instance exists
	instance, err := manager.GetInstance(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %v", err)
	}

	// Delete the instance
	if err := manager.DeleteInstance(instanceID); err != nil {
		return fmt.Errorf("failed to delete instance: %v", err)
	}

	m.log.Info("Successfully deleted instance %s (%s)", instanceID, instance.Name)
	return nil
}
