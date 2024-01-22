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

package dryrun

import (
	"fmt"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

type options struct {
	envFile string

	cfg v1alpha1.Environment
}

type command struct {
	logger *logrus.Logger
}

// NewCommand constructs a dryrun command with the specified logger
func NewCommand(logger *logrus.Logger) *cli.Command {
	c := command{
		logger: logger,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	opts := options{}

	// Create the 'dryrun' command
	dryrun := cli.Command{
		Name:  "dryrun",
		Usage: "dryrun a test environment based on config file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "envFile",
				Aliases:     []string{"f"},
				Usage:       "Path to the Environment file",
				Destination: &opts.envFile,
			},
		},
		Before: func(c *cli.Context) error {
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.envFile)
			if err != nil {
				fmt.Printf("failed to read config file: %v\n", err)
				return err
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			return m.run(c, &opts)
		},
	}

	return &dryrun
}

func (m command) run(c *cli.Context, opts *options) error {
	// Check Provider
	switch opts.cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		err := validateAWS(opts)
		if err != nil {
			return err
		}
	case v1alpha1.ProviderSSH:
		// Creating a new provisioner will validate the private key and hostUrl
		_, err := provisioner.New(opts.cfg.Spec.Auth.PrivateKey, opts.cfg.Spec.Instance.HostUrl)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown provider %s", opts.cfg.Spec.Provider)
	}

	// Check Provisioner
	err := provisioner.Dryrun(opts.cfg)
	if err != nil {
		return err
	}

	fmt.Printf("Dryrun succeeded\n")

	return nil
}

func validateAWS(opts *options) error {
	client, err := aws.New(opts.cfg, opts.envFile)
	if err != nil {
		return err
	}

	if err = client.DryRun(); err != nil {
		return err
	}

	return nil
}
