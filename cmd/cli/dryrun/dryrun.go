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
	"os"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	cli "github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

type options struct {
	envFile string

	cfg v1alpha1.Environment
}

type command struct {
	log *logger.FunLogger
}

// NewCommand constructs the DryRun command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
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
				return fmt.Errorf("failed to read config file %s: %w", opts.envFile, err)
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			return m.run(&opts)
		},
	}

	return &dryrun
}

func (m command) run(opts *options) error {
	m.log.Info("Dryrun environment %s \U0001f50d", opts.cfg.Name)

	// Check Provider
	switch opts.cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		err := validateAWS(m.log, opts)
		if err != nil {
			return err
		}
	case v1alpha1.ProviderSSH:
		// if username is not provided, use the current user
		if opts.cfg.Spec.Username == "" {
			opts.cfg.Spec.Username = os.Getenv("USER")
		}
		if err := connectOrDie(opts.cfg.Spec.PrivateKey, opts.cfg.Spec.Username, opts.cfg.Spec.HostUrl); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown provider %s", opts.cfg.Spec.Provider)
	}

	// Check Provisioner
	if err := provisioner.Dryrun(m.log, opts.cfg); err != nil {
		return err
	}

	m.log.Info("Dryrun succeeded \U0001F389")

	return nil
}

func validateAWS(log *logger.FunLogger, opts *options) error {
	client, err := aws.New(log, opts.cfg, opts.envFile)
	if err != nil {
		return err
	}

	if err = client.DryRun(); err != nil {
		return err
	}

	return nil
}

// createSshClient creates a ssh client, and retries if it fails to connect
func connectOrDie(keyPath, userName, hostUrl string) error {
	var err error
	key, err := os.ReadFile(keyPath) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	sshConfig := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint:gosec
	}

	connectionFailed := false
	for range 20 {
		client, err := ssh.Dial("tcp", hostUrl+":22", sshConfig)
		if err == nil {
			client.Close() // nolint:errcheck, gosec
			return nil     // Connection succeeded,
		}
		connectionFailed = true
		// Sleep for a brief moment before retrying.
		// You can adjust the duration based on your requirements.
		time.Sleep(1 * time.Second)
	}

	if connectionFailed {
		return fmt.Errorf("failed to connect to %s", hostUrl)
	}

	return nil
}
