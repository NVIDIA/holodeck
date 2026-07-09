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

package main

import (
	"context"
	"os"

	"github.com/NVIDIA/holodeck/cmd/cli/cleanup"
	"github.com/NVIDIA/holodeck/cmd/cli/create"
	"github.com/NVIDIA/holodeck/cmd/cli/delete"
	"github.com/NVIDIA/holodeck/cmd/cli/describe"
	"github.com/NVIDIA/holodeck/cmd/cli/dryrun"
	"github.com/NVIDIA/holodeck/cmd/cli/get"
	"github.com/NVIDIA/holodeck/cmd/cli/list"
	oscmd "github.com/NVIDIA/holodeck/cmd/cli/os"
	"github.com/NVIDIA/holodeck/cmd/cli/scp"
	"github.com/NVIDIA/holodeck/cmd/cli/skill"
	"github.com/NVIDIA/holodeck/cmd/cli/ssh"
	"github.com/NVIDIA/holodeck/cmd/cli/status"
	"github.com/NVIDIA/holodeck/cmd/cli/update"
	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v3"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "holodeck"
)

// ProgramVersion is the program version. It is set at build time via
// -ldflags "-X main.ProgramVersion=..." (see .goreleaser.yaml).
var ProgramVersion = "dev"

type config struct {
	Debug   bool
	Verbose bool
	Quiet   bool
}

// NewApp creates and configures the CLI application.
func NewApp(log *logger.FunLogger) *cli.Command {
	cfg := config{}

	c := &cli.Command{}
	c.Name = ProgramName
	c.Usage = "Create and manage test environments"
	c.Description = `
Holodeck is a tool for creating and managing test environments.
It supports multiple providers (AWS, SSH) and can provision environments with
various container runtimes and Kubernetes distributions.

Examples:
  # Create a new environment from a config file
  holodeck create -f env.yaml

  # Create and provision an environment
  holodeck create -f env.yaml --provision

  # List all environments
  holodeck list

  # List environments in JSON format
  holodeck list -o json

  # Get status of a specific environment
  holodeck status <instance-id>

  # SSH into an instance
  holodeck ssh <instance-id>

  # Run a command on an instance
  holodeck ssh <instance-id> -- nvidia-smi

  # Copy files to/from an instance
  holodeck scp ./local-file.txt <instance-id>:/remote/path/
  holodeck scp <instance-id>:/remote/file.log ./local/

  # Delete an environment
  holodeck delete <instance-id>

  # Clean up AWS VPC resources
  holodeck cleanup vpc-12345678

  # Use a custom cache directory
  holodeck create -f env.yaml --cachepath /path/to/cache`
	c.Version = ProgramVersion
	c.EnableShellCompletion = true

	// Setup the flags for this command
	c.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "quiet",
			Aliases:     []string{"q"},
			Usage:       "Suppress non-error output",
			Destination: &cfg.Quiet,
		},
		&cli.BoolFlag{
			Name:        "verbose",
			Usage:       "Enable verbose output",
			Destination: &cfg.Verbose,
		},
		&cli.BoolFlag{
			Name:        "debug",
			Aliases:     []string{"d"},
			Usage:       "Enable debug-level logging",
			Destination: &cfg.Debug,
			Sources:     cli.EnvVars("DEBUG"),
		},
	}

	// Set verbosity based on flags (precedence: debug > verbose > quiet)
	c.Before = func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		switch {
		case cmd.Bool("debug"):
			log.SetVerbosity(logger.VerbosityDebug)
		case cmd.Bool("verbose"):
			log.SetVerbosity(logger.VerbosityVerbose)
		case cmd.Bool("quiet"):
			log.SetVerbosity(logger.VerbosityQuiet)
		default:
			log.SetVerbosity(logger.VerbosityNormal)
		}
		return ctx, nil
	}

	// Define the subcommands
	c.Commands = []*cli.Command{
		cleanup.NewCommand(log),
		create.NewCommand(log),
		delete.NewCommand(log),
		describe.NewCommand(log),
		dryrun.NewCommand(log),
		get.NewCommand(log),
		list.NewCommand(log),
		oscmd.NewCommand(log),
		scp.NewCommand(log),
		skill.NewCommand(log),
		ssh.NewCommand(log),
		status.NewCommand(log),
		update.NewCommand(log),
	}

	return c
}

func main() {
	log := logger.NewLogger()
	c := NewApp(log)

	// Custom help template
	c.CustomRootCommandHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.FullName}} [global options] command [command options] [arguments...]

VERSION:
   {{.Version}}

DESCRIPTION:
   {{.Description}}

COMMANDS:
{{range .VisibleCommands}}   {{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}

GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}

EXAMPLES:
   # Create a new environment from a config file
   {{.Name}} create -f env.yaml

   # Create and provision an environment
   {{.Name}} create -f env.yaml --provision

   # List all environments
   {{.Name}} list

   # List environments in JSON format
   {{.Name}} list -o json

   # Get status of a specific environment
   {{.Name}} status <instance-id>

   # SSH into an instance
   {{.Name}} ssh <instance-id>

   # Run a command on an instance
   {{.Name}} ssh <instance-id> -- nvidia-smi

   # Copy files to/from an instance
   {{.Name}} scp ./local-file.txt <instance-id>:/remote/path/

   # Delete an environment
   {{.Name}} delete <instance-id>

   # Clean up AWS VPC resources
   {{.Name}} cleanup vpc-12345678

For more information about a command, run:
   {{.Name}} help <command>
`

	err := c.Run(context.Background(), os.Args)
	if err != nil {
		log.Error(err)
		log.Exit(1)
	}
}
