/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package ssh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cli "github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/common"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
	node      string
}

// NewCommand constructs the ssh command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	// Create the 'ssh' command
	sshCmd := cli.Command{
		Name:      "ssh",
		Usage:     "SSH into a Holodeck instance",
		ArgsUsage: "<instance-id> [-- <command>]",
		Description: `Connect to a Holodeck instance via SSH.

Examples:
  # Interactive shell
  holodeck ssh abc123

  # Run a single command
  holodeck ssh abc123 -- nvidia-smi

  # Run kubectl on the instance
  holodeck ssh abc123 -- kubectl get nodes

  # For multinode clusters, specify a node
  holodeck ssh abc123 --node worker-0 -- nvidia-smi

Note: Remote commands are joined with spaces and passed to the remote shell.
If you need special quoting, wrap the entire command in single quotes:
  holodeck ssh abc123 -- 'echo "hello world"'`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &m.cachePath,
			},
			&cli.StringFlag{
				Name:        "node",
				Aliases:     []string{"n"},
				Usage:       "Node name for multinode clusters (default: first control-plane)",
				Destination: &m.node,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("instance ID is required")
			}
			instanceID := cmd.Args().Get(0)

			//nolint:contextcheck // run -> common.ConnectSSH is a CLI action boundary with no ctx parameter by design (public signature is locked); threading requires a signature change out of scope here.
			return m.run(instanceID, remoteCommand(cmd.Args()))
		},
	}

	return &sshCmd
}

// remoteCommand returns the command to run on the remote host, or nil for an
// interactive session. In urfave/cli v3 the "--" terminator is consumed by the
// flag parser (it stops flag parsing and is dropped from the positional args),
// so every positional argument after the instance ID constitutes the remote
// command. This preserves the documented `ssh <id> -- <command>` behaviour
// exactly: the "--" still protects the remote command's own flags from being
// parsed as ssh flags.
func remoteCommand(args cli.Args) []string {
	return args.Tail()
}

func (m command) run(instanceID string, remoteCmd []string) error {
	// Get instance details
	manager := instances.NewManager(m.log, m.cachePath)
	instance, err := manager.GetInstance(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	// Load environment for SSH details
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](instance.CacheFile)
	if err != nil {
		return fmt.Errorf("failed to read environment: %w", err)
	}

	// Determine host URL
	hostUrl, err := common.GetHostURL(&env, m.node, true)
	if err != nil {
		return fmt.Errorf("failed to get host URL: %w", err)
	}

	// Get SSH credentials from environment
	keyPath := env.Spec.PrivateKey
	userName := env.Spec.Username
	if userName == "" {
		userName = "ubuntu"
	}

	// For interactive sessions, use system SSH for better terminal support
	if len(remoteCmd) == 0 {
		return m.runInteractiveSystemSSH(keyPath, userName, hostUrl)
	}

	// For command execution, use Go SSH library
	client, err := common.ConnectSSH(m.log, keyPath, userName, hostUrl)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close() //nolint:errcheck

	return m.runCommand(client, remoteCmd)
}

func (m command) runCommand(client *ssh.Client, cmd []string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close() //nolint:errcheck

	// Connect stdout and stderr
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// SSH remote execution always passes the command through the remote shell,
	// so we join arguments with spaces. Users who need literal quoting should
	// wrap arguments in shell quotes themselves (e.g., -- 'echo "hello"').
	cmdStr := strings.Join(cmd, " ")

	return session.Run(cmdStr)
}

// runInteractiveSystemSSH uses the system's ssh command for interactive sessions
// This provides better terminal support (colors, window resize, etc.)
func (m command) runInteractiveSystemSSH(keyPath, userName, hostUrl string) error {
	// Use holodeck's known_hosts file for TOFU-consistent host key verification.
	cacheBase, _ := os.UserCacheDir()
	knownHostsPath := filepath.Join(cacheBase, "holodeck", "known_hosts")

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", userName, hostUrl),
	}

	cmd := exec.Command("ssh", args...) //nolint:gosec // args are constructed from trusted env config
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
