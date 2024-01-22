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
	"os"

	"github.com/NVIDIA/holodeck/cmd/create"
	"github.com/NVIDIA/holodeck/cmd/delete"
	"github.com/NVIDIA/holodeck/cmd/dryrun"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "holodeck"
)

var logger = log.New()

type config struct {
	Debug bool
}

func main() {
	config := config{}

	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = ProgramName
	c.Usage = "Create and manage test environments"
	c.Version = "0.1.0"

	// Setup the flags for this command
	c.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Aliases:     []string{"d"},
			Usage:       "Enable debug-level logging",
			Destination: &config.Debug,
			EnvVars:     []string{"DEBUG"},
		},
	}

	// Set log-level for all subcommands
	c.Before = func(c *cli.Context) error {
		logLevel := log.InfoLevel
		if config.Debug {
			logLevel = log.DebugLevel
		}
		logger.SetLevel(logLevel)
		return nil
	}

	// Define the subcommands
	c.Commands = []*cli.Command{
		create.NewCommand(logger),
		delete.NewCommand(logger),
		dryrun.NewCommand(logger),
	}

	err := c.Run(os.Args)
	if err != nil {
		log.Errorf("%v", err)
		log.Exit(1)
	}
}
