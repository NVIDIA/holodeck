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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/pkg/sshutil"

	cli "github.com/urfave/cli/v3"
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
		Before: func(ctx context.Context, _ *cli.Command) (context.Context, error) {
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.envFile)
			if err != nil {
				return ctx, fmt.Errorf("failed to read config file %s: %w", opts.envFile, err)
			}

			return ctx, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			//nolint:contextcheck // provisioner.Dryrun -> logger.Loading (pkg/provisioner, internal/logger) have no ctx parameter by design; threading requires a signature change outside cmd/cli, out of scope here.
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

// dryrunDialer builds the sshutil.Dialer used by connectOrDie. Extracted so
// it is independently testable: it must carry a non-zero handshake timeout
// (the reported bug — dryrun previously set none and could block forever on
// an unresponsive host) while keeping the historical 20x1s retry envelope.
func dryrunDialer(keyPath, userName string, log *logger.FunLogger) *sshutil.Dialer {
	return &sshutil.Dialer{
		Auth:     sshutil.AuthConfig{User: userName, KeyPath: keyPath},
		HostKey:  sshutil.HostKeyPolicyAcceptNew,
		Retry:    sshutil.RetryPolicy{MaxAttempts: 20, Delay: 1 * time.Second},
		Timeouts: sshutil.TimeoutConfig{Handshake: 15 * time.Second},
		Log:      log,
	}
}

// connectOrDie creates a ssh client, and retries if it fails to connect.
func connectOrDie(keyPath, userName, hostUrl string) error {
	client, err := dryrunDialer(keyPath, userName, logger.NewLogger()).Dial(context.Background(), hostUrl, nil) //nolint:contextcheck // CLI action boundary; no ctx to thread yet
	if err != nil {
		return err
	}
	_ = client.Close()
	return nil
}
