/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

package cleanup

import (
	"fmt"
	"os"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/cleanup"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log         *logger.FunLogger
	region      string
	forceDelete bool
}

// NewCommand constructs the cleanup command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := &command{
		log: log,
	}
	return c.build()
}

func (m *command) build() *cli.Command {
	// Create the 'cleanup' command
	cleanup := cli.Command{
		Name:  "cleanup",
		Usage: "Clean up AWS VPC resources",
		Description: `Clean up AWS VPC resources by VPC ID.

This command will:
- Check GitHub job status (if GITHUB_TOKEN is set and tags are present)
- Delete all resources in the VPC including:
  * EC2 instances
  * Security groups
  * Subnets
  * Route tables
  * Internet gateways
  * The VPC itself

Examples:
  # Clean up a single VPC
  holodeck cleanup vpc-12345678

  # Clean up multiple VPCs
  holodeck cleanup vpc-12345678 vpc-87654321

  # Force cleanup without job status check
  holodeck cleanup --force vpc-12345678

  # Clean up in a specific region
  holodeck cleanup --region us-west-2 vpc-12345678`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "region",
				Aliases:     []string{"r"},
				Usage:       "AWS region (overrides AWS_REGION env var)",
				Destination: &m.region,
			},
			&cli.BoolFlag{
				Name:        "force",
				Aliases:     []string{"f"},
				Usage:       "Force cleanup without checking job status",
				Destination: &m.forceDelete,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return fmt.Errorf("at least one VPC ID is required")
			}
			return m.run(c)
		},
	}

	return &cleanup
}

func (m *command) run(c *cli.Context) error {
	// Determine the region
	region := m.region
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
			if region == "" {
				return fmt.Errorf("AWS region must be specified via --region flag or AWS_REGION environment variable")
			}
		}
	}

	// Create the cleaner
	cleaner, err := cleanup.New(m.log, region)
	if err != nil {
		return fmt.Errorf("failed to create cleaner: %w", err)
	}

	// Process each VPC ID
	successCount := 0
	failCount := 0

	for _, vpcID := range c.Args().Slice() {
		m.log.Info("Processing VPC: %s", vpcID)

		var err error
		if m.forceDelete {
			// Skip job status check
			err = cleaner.DeleteVPCResources(vpcID)
		} else {
			// Check job status first
			err = cleaner.CleanupVPC(vpcID)
		}

		if err != nil {
			m.log.Error(fmt.Errorf("failed to cleanup VPC %s: %v", vpcID, err))
			failCount++
		} else {
			m.log.Info("Successfully cleaned up VPC %s", vpcID)
			successCount++
		}
	}

	if failCount > 0 {
		return fmt.Errorf("cleanup completed with errors: %d succeeded, %d failed", successCount, failCount)
	}

	m.log.Info("Cleanup completed successfully: %d VPCs cleaned up", successCount)
	return nil
}
