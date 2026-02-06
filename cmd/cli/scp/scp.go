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

package scp

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/common"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log       *logger.FunLogger
	cachePath string
	node      string
	recursive bool
}

// NewCommand constructs the scp command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	// Create the 'scp' command
	scpCmd := cli.Command{
		Name:      "scp",
		Usage:     "Copy files to/from a Holodeck instance",
		ArgsUsage: "<source> <destination>",
		Description: `Copy files to or from a Holodeck instance using SFTP.

Use <instance-id>:<path> syntax to specify remote paths.

Examples:
  # Copy local file to remote instance
  holodeck scp ./local-file.txt abc123:/home/ubuntu/

  # Copy remote file to local directory
  holodeck scp abc123:/var/log/syslog ./logs/

  # Copy directory recursively
  holodeck scp -r ./config/ abc123:/home/ubuntu/config/

  # For multinode clusters, specify a node
  holodeck scp --node worker-0 ./script.sh abc123:/tmp/`,
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
			&cli.BoolFlag{
				Name:        "recursive",
				Aliases:     []string{"r"},
				Usage:       "Copy directories recursively",
				Destination: &m.recursive,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 2 {
				return fmt.Errorf("source and destination are required")
			}
			return m.run(c.Args().Get(0), c.Args().Get(1))
		},
	}

	return &scpCmd
}

// pathSpec represents a parsed path (local or remote)
type pathSpec struct {
	instanceID string
	path       string
	isRemote   bool
}

func parsePath(path string) pathSpec {
	// Check for instance-id:path format
	if idx := strings.Index(path, ":"); idx > 0 {
		// Make sure it's not a Windows path like C:\
		if idx == 1 && len(path) > 2 && path[2] == '\\' {
			return pathSpec{path: path, isRemote: false}
		}
		return pathSpec{
			instanceID: path[:idx],
			path:       path[idx+1:],
			isRemote:   true,
		}
	}
	return pathSpec{path: path, isRemote: false}
}

func (m command) run(src, dst string) error {
	srcSpec := parsePath(src)
	dstSpec := parsePath(dst)

	// Validate that exactly one is remote
	if srcSpec.isRemote == dstSpec.isRemote {
		if srcSpec.isRemote {
			return fmt.Errorf("cannot copy between two remote locations")
		}
		return fmt.Errorf("one of source or destination must be a remote path (instance-id:path)")
	}

	// Get instance ID from the remote spec
	var instanceID string
	if srcSpec.isRemote {
		instanceID = srcSpec.instanceID
	} else {
		instanceID = dstSpec.instanceID
	}

	// Get instance details
	manager := instances.NewManager(m.log, m.cachePath)
	instance, err := manager.GetInstance(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %v", err)
	}

	// Load environment for SSH details
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](instance.CacheFile)
	if err != nil {
		return fmt.Errorf("failed to read environment: %v", err)
	}

	// Determine host URL
	hostUrl, err := common.GetHostURL(&env, m.node, true)
	if err != nil {
		return fmt.Errorf("failed to get host URL: %v", err)
	}

	// Get SSH credentials
	keyPath := env.Spec.PrivateKey
	userName := env.Spec.Username
	if userName == "" {
		userName = "ubuntu"
	}

	// Create SSH and SFTP clients
	sshClient, err := common.ConnectSSH(m.log, keyPath, userName, hostUrl)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer sshClient.Close() //nolint:errcheck

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close() //nolint:errcheck

	// Perform the copy
	if srcSpec.isRemote {
		return m.copyFromRemote(sftpClient, srcSpec.path, dstSpec.path)
	}
	return m.copyToRemote(sftpClient, srcSpec.path, dstSpec.path)
}

func (m command) copyToRemote(client *sftp.Client, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local path: %v", err)
	}

	if info.IsDir() {
		if !m.recursive {
			return fmt.Errorf("%s is a directory, use -r for recursive copy", localPath)
		}
		return m.copyDirToRemote(client, localPath, remotePath)
	}

	return m.copyFileToRemote(client, localPath, remotePath)
}

func (m command) copyFileToRemote(client *sftp.Client, localPath, remotePath string) error {
	// Open local file
	localFile, err := os.Open(localPath) //nolint:gosec // localPath is user-provided CLI arg
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close() //nolint:errcheck

	// Ensure remote directory exists (use path, not filepath, for POSIX remote paths)
	remoteDir := path.Dir(remotePath)
	_ = client.MkdirAll(remoteDir) // best-effort, directory may already exist

	// Create remote file
	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close() //nolint:errcheck

	// Copy content
	bytes, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	m.log.Info("Copied %s -> %s (%d bytes)", localPath, remotePath, bytes)
	return nil
}

func (m command) copyDirToRemote(client *sftp.Client, localPath, remotePath string) error {
	return filepath.Walk(localPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(localPath, walkPath)
		if err != nil {
			return err
		}

		remoteTarget := path.Join(remotePath, relPath)

		if info.IsDir() {
			return client.MkdirAll(remoteTarget)
		}

		return m.copyFileToRemote(client, walkPath, remoteTarget)
	})
}

func (m command) copyFromRemote(client *sftp.Client, remotePath, localPath string) error {
	info, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("failed to stat remote path: %v", err)
	}

	if info.IsDir() {
		if !m.recursive {
			return fmt.Errorf("%s is a directory, use -r for recursive copy", remotePath)
		}
		return m.copyDirFromRemote(client, remotePath, localPath)
	}

	return m.copyFileFromRemote(client, remotePath, localPath)
}

func (m command) copyFileFromRemote(client *sftp.Client, remotePath, localPath string) error {
	// Open remote file
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFile.Close() //nolint:errcheck

	// Ensure local directory exists
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0750); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Create local file
	localFile, err := os.Create(localPath) //nolint:gosec // localPath is user-provided CLI arg
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close() //nolint:errcheck

	// Copy content
	bytes, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	m.log.Info("Copied %s -> %s (%d bytes)", remotePath, localPath, bytes)
	return nil
}

func (m command) copyDirFromRemote(client *sftp.Client, remotePath, localPath string) error {
	walker := client.Walk(remotePath)

	for walker.Step() {
		if walker.Err() != nil {
			m.log.Warning("Skipping %s: %v", walker.Path(), walker.Err())
			continue
		}

		// Calculate relative path (remote paths are POSIX)
		relPath := strings.TrimPrefix(walker.Path(), remotePath)
		relPath = strings.TrimPrefix(relPath, "/")

		localTarget := filepath.Join(localPath, relPath)

		if walker.Stat().IsDir() {
			if err := os.MkdirAll(localTarget, 0750); err != nil {
				return err
			}
			continue
		}

		if err := m.copyFileFromRemote(client, walker.Path(), localTarget); err != nil {
			return err
		}
	}

	return nil
}
