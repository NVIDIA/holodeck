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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/cleanup"

	cli "github.com/urfave/cli/v2"
)

// Default timeout for cleanup operations (15 minutes per VPC)
const defaultCleanupTimeout = 15 * time.Minute

type command struct {
	log         *logger.FunLogger
	region      string
	forceDelete bool
	timeout     time.Duration
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
  holodeck cleanup --region us-west-2 vpc-12345678

  # Clean up with custom timeout (per VPC)
  holodeck cleanup --timeout 30m vpc-12345678`,
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
			&cli.DurationFlag{
				Name:        "timeout",
				Aliases:     []string{"t"},
				Usage:       "Timeout per VPC cleanup operation (default: 15m)",
				Value:       defaultCleanupTimeout,
				Destination: &m.timeout,
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

	// Set default timeout if not specified
	timeout := m.timeout
	if timeout == 0 {
		timeout = defaultCleanupTimeout
	}

	// Create a context that can be cancelled by SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		m.log.Warning("Received signal %v, cancelling cleanup operations...", sig)
		cancel()
	}()

	// Create the cleaner
	cleaner, err := cleanup.New(m.log, region)
	if err != nil {
		return fmt.Errorf("failed to create cleaner: %w", err)
	}

	// Process each VPC ID
	successCount := 0
	failCount := 0

	for _, vpcID := range c.Args().Slice() {
		// Check if context was cancelled before starting next VPC
		if ctx.Err() != nil {
			m.log.Warning("Cleanup cancelled, skipping remaining VPCs")
			break
		}

		m.log.Info("Processing VPC: %s (timeout: %v)", vpcID, timeout)

		// Create a context with timeout for this specific VPC cleanup
		vpcCtx, vpcCancel := context.WithTimeout(ctx, timeout)

		var cleanupErr error
		if m.forceDelete {
			// Skip job status check
			cleanupErr = cleaner.DeleteVPCResources(vpcCtx, vpcID)
		} else {
			// Check job status first
			cleanupErr = cleaner.CleanupVPC(vpcCtx, vpcID)
		}

		vpcCancel() // Clean up the VPC-specific context

		if cleanupErr != nil {
			if ctx.Err() != nil {
				m.log.Error(fmt.Errorf("cleanup of VPC %s was cancelled: %v", vpcID, cleanupErr))
			} else {
				m.log.Error(fmt.Errorf("failed to cleanup VPC %s: %v", vpcID, cleanupErr))
			}
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
