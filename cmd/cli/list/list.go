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
	"os"
	"text/tabwriter"
	"time"

	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
}

// NewCommand constructs the list command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
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
		},
		Action: m.run,
	}

	return &list
}

func (m command) run(c *cli.Context) error {
	manager := instances.NewManager(m.log, m.cachePath)
	instances, err := manager.ListInstances()
	if err != nil {
		return fmt.Errorf("failed to list instances: %v", err)
	}

	if len(instances) == 0 {
		m.log.Info("No instances found")
		return nil
	}

	// Create a tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "INSTANCE ID\tNAME\tPROVIDER\tSTATUS\tPROVISIONED\tCREATED\tAGE"); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	for _, instance := range instances {
		// Skip instances without an ID (old cache files)
		if instance.ID == "" {
			m.log.Warning("Found old cache file without instance ID, skipping: %s", instance.CacheFile)
			continue
		}

		age := time.Since(instance.CreatedAt).Round(time.Second)
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\t%s\t%s\n",
			instance.ID, // Show full instance ID
			instance.Name,
			instance.Provider,
			instance.Status,
			instance.Provisioned,
			instance.CreatedAt.Format("2006-01-02 15:04:05"),
			age,
		); err != nil {
			return fmt.Errorf("failed to write instance data: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %v", err)
	}

	return nil
}
