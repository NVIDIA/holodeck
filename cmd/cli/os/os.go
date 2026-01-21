/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

// Package os provides CLI commands for managing operating system images.
package os

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/NVIDIA/holodeck/internal/ami"
	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	cli "github.com/urfave/cli/v2"
)

type command struct {
	log *logger.FunLogger
}

// NewCommand constructs the os command with the specified logger.
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := &command{
		log: log,
	}
	return c.build()
}

func (c *command) build() *cli.Command {
	return &cli.Command{
		Name:  "os",
		Usage: "Manage operating system images",
		Description: `Commands for listing and querying supported operating systems.

Holodeck supports automatic AMI resolution based on OS identifiers.
Instead of specifying a specific AMI ID, you can use the 'os' field
in your environment configuration:

  spec:
    instance:
      os: ubuntu-22.04  # AMI auto-resolved based on region

Use these commands to discover available operating systems and their
corresponding AMI IDs for specific regions.`,
		Subcommands: []*cli.Command{
			c.buildListCommand(),
			c.buildDescribeCommand(),
			c.buildAMICommand(),
		},
	}
}

func (c *command) buildListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available operating systems",
		Description: `List all operating systems supported by Holodeck.

The output shows the OS identifier (used in configuration files),
the OS family, default SSH username, and supported architectures.

Example:
  holodeck os list`,
		Action: c.runList,
	}
}

func (c *command) runList(_ *cli.Context) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tFAMILY\tSSH USER\tPACKAGE MGR\tARCHITECTURES\tNOTES"); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	for _, img := range ami.All() {
		notes := ""
		// containerd now supports DNF/YUM, other components may still need work
		if img.PackageManager != ami.PackageManagerAPT {
			notes = "⚠ partial support (containerd only)"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			img.ID,
			img.Family,
			img.SSHUsername,
			img.PackageManager,
			strings.Join(img.Architectures, ", "),
			notes,
		); err != nil {
			return fmt.Errorf("error writing OS info: %w", err)
		}
	}
	return w.Flush()
}

func (c *command) buildDescribeCommand() *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Usage:     "Show details for a specific operating system",
		ArgsUsage: "<os-id>",
		Description: `Display detailed information about a specific operating system.

Example:
  holodeck os describe ubuntu-22.04`,
		Action: c.runDescribe,
	}
}

func (c *command) runDescribe(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("OS identifier required (run 'holodeck os list' for options)")
	}

	osID := ctx.Args().First()
	img, ok := ami.Get(osID)
	if !ok {
		return fmt.Errorf(
			"unknown OS: %s (run 'holodeck os list' for available options)",
			osID,
		)
	}

	fmt.Printf("ID:              %s\n", img.ID)
	fmt.Printf("Name:            %s\n", img.Name)
	fmt.Printf("Family:          %s\n", img.Family)
	fmt.Printf("SSH Username:    %s\n", img.SSHUsername)
	fmt.Printf("Package Manager: %s\n", img.PackageManager)
	fmt.Printf("Min Root Volume: %d GB\n", img.MinRootVolumeGB)
	fmt.Printf("Architectures:   %s\n", strings.Join(img.Architectures, ", "))
	fmt.Printf("AWS Owner ID:    %s\n", img.OwnerID)

	if img.SSMPath != "" {
		fmt.Printf("SSM Path:        %s\n", img.SSMPath)
	}

	return nil
}

func (c *command) buildAMICommand() *cli.Command {
	var region, arch string

	return &cli.Command{
		Name:      "ami",
		Usage:     "Get the AMI ID for an OS in a specific region",
		ArgsUsage: "<os-id>",
		Description: `Resolve the AMI ID for a specific operating system.

This command queries AWS to find the latest AMI for the specified
OS, region, and architecture.

Examples:
  # Get AMI for ubuntu-22.04 in us-west-2
  holodeck os ami ubuntu-22.04 --region us-west-2

  # Get AMI for arm64 architecture
  holodeck os ami ubuntu-22.04 --region us-west-2 --arch arm64`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "region",
				Aliases:     []string{"r"},
				Usage:       "AWS region (required)",
				Destination: &region,
				Required:    true,
				EnvVars:     []string{"AWS_REGION"},
			},
			&cli.StringFlag{
				Name:        "arch",
				Aliases:     []string{"a"},
				Usage:       "CPU architecture (x86_64 or arm64)",
				Destination: &arch,
				Value:       "x86_64",
			},
		},
		Action: func(ctx *cli.Context) error {
			return c.runAMI(ctx, region, arch)
		},
	}
}

func (c *command) runAMI(ctx *cli.Context, region, arch string) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("OS identifier required (run 'holodeck os list' for options)")
	}

	osID := ctx.Args().First()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create clients
	ec2Client := ec2.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	// Create resolver and resolve
	resolver := ami.NewResolver(ec2Client, ssmClient, region)
	resolved, err := resolver.Resolve(context.Background(), osID, arch)
	if err != nil {
		return err
	}

	fmt.Println(resolved.ImageID)
	return nil
}
