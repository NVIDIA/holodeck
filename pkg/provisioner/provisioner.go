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

package provisioner

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

const Shebang = `#! /usr/bin/env bash
set -xe
`

const remoteKindConfig = "/etc/kubernetes/kind.yaml"

type Provisioner struct {
	Client         *ssh.Client
	SessionManager *ssm.Client

	HostUrl  string
	UserName string
	KeyPath  string
	tpl      bytes.Buffer

	log *logger.FunLogger
}

func New(log *logger.FunLogger, keyPath, userName, hostUrl string) (*Provisioner, error) {
	client, err := connectOrDie(keyPath, userName, hostUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", hostUrl, err)
	}

	p := &Provisioner{
		Client:   client,
		HostUrl:  hostUrl,
		UserName: userName,
		KeyPath:  keyPath,
		tpl:      bytes.Buffer{},
		log:      log,
	}

	return p, nil
}

// waitForNodeReboot waits for the node to reboot and come back online after a kernel version change
func (p *Provisioner) waitForNodeReboot() error {
	p.log.Info("Kernel version change detected, waiting for node to reboot...")

	// Check if the connection is still active before closing
	if p.Client != nil {
		// Try to create a new session to check if connection is alive
		session, err := p.Client.NewSession()
		if err == nil {
			session.Close() // nolint:errcheck, gosec
			// Connection is alive, close it
			if err := p.Client.Close(); err != nil {
				return fmt.Errorf("failed to close ssh client: %w", err)
			}
		}
		// If we get here, either the connection was already closed or we couldn't create a session
		p.Client = nil
	}

	// Wait for the node to come back up
	maxRetries := 30
	retryInterval := 10 * time.Second

	for i := 0; i < maxRetries; i++ {
		p.log.Info("Waiting for node to come back online...")
		time.Sleep(retryInterval)

		// Try to establish a new connection
		client, err := connectOrDie(p.KeyPath, p.UserName, p.HostUrl)
		if err == nil {
			p.Client = client
			p.log.Info("Node is back online, continuing with provisioning...")
			return nil
		}

		if i == maxRetries-1 {
			return fmt.Errorf("node did not come back online after %d attempts", maxRetries)
		}
	}

	return nil
}

func (p *Provisioner) Run(env v1alpha1.Environment) error {
	dependencies := NewDependencies(env)

	// Create kubeadm config file if required installer is kubeadm and not using legacy mode
	if env.Spec.Kubernetes.Installer == "kubeadm" {
		// Set the k8s endpoint host to the host url
		env.Spec.Kubernetes.K8sEndpointHost = p.HostUrl

		// Check if we need to use legacy mode
		kubernetes, err := templates.NewKubernetes(env)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes template: %v", err)
		}

		// Only create kubeadm config file if not using legacy mode
		if !kubernetes.UseLegacyInit {
			if err := p.createKubeAdmConfig(env); err != nil {
				return fmt.Errorf("failed to create kubeadm config file: %v", err)
			}
		}
	}

	// kind-config
	// Create kind config file if it is provided
	if (env.Spec.Kubernetes.Installer == "kind" || env.Spec.Kubernetes.Installer == "nvkind") && env.Spec.Kubernetes.KindConfig != "" {
		if err := p.createKindConfig(env); err != nil {
			return fmt.Errorf("failed to create kind config file: %v", err)
		}
	}

	if env.Spec.Kubernetes.Installer == "kubeadm" {
		env.Spec.Kubernetes.K8sEndpointHost = p.HostUrl
	}

	for _, node := range dependencies.Resolve() {
		// Add script header and common functions to the script
		if err := addScriptHeader(&p.tpl); err != nil {
			return fmt.Errorf("failed to add shebang to the script: %v", err)
		}
		// Execute the template for the dependency
		if err := node(&p.tpl, env); err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
		// Provision the instance
		if err := p.provision(); err != nil {
			return fmt.Errorf("failed to provision: %v", err)
		}

		// If kernel version is specified, wait for the node to reboot
		if env.Spec.Kernel.Version != "" {
			if err := p.waitForNodeReboot(); err != nil {
				return err
			}
		} else {
			// Reset the connection, this step is needed to make sure some configuration changes take effect
			// e.g after installing docker, the user needs to be added to the docker group
			if err := p.resetConnection(); err != nil {
				return fmt.Errorf("failed to reset connection: %w", err)
			}
		}

		// Clear the template buffer
		p.tpl.Reset()
	}

	return nil
}

// resetConnection resets the ssh connection, and retries if it fails to connect
func (p *Provisioner) resetConnection() error {
	// Close the current ssh connection
	if err := p.Client.Close(); err != nil {
		return fmt.Errorf("failed to close ssh client: %v", err)
	}

	return nil
}

func (p *Provisioner) provision() error {
	var err error

	// Create a new ssh connection
	p.Client, err = connectOrDie(p.KeyPath, p.UserName, p.HostUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", p.HostUrl, err)
	}

	// Create a session
	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	reader, writer := io.Pipe()
	session.Stdout = writer
	session.Stderr = writer

	go func() {
		defer writer.Close() // nolint:errcheck, gosec
		_, err := io.Copy(os.Stdout, reader)
		if err != nil {
			log.Fatalf("Failed to copy from reader: %v", err)
		}
	}()
	defer session.Close() // nolint:errcheck, gosec

	script := p.tpl.String()

	// run the script
	err = session.Start(script)
	if err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}
	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for session: %v", err)
	}

	return nil
}

func (p *Provisioner) createKindConfig(env v1alpha1.Environment) error {
	// Specify the remote file path
	remoteFilePath := remoteKindConfig

	// Create a session
	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close() // nolint:errcheck, gosec

	// create remote directory if it does not exist
	if err := session.Run("sudo mkdir -p /etc/kubernetes"); err != nil {
		return fmt.Errorf("failed to create remote directory /etc/kubernetes: %v", err)
	}

	// Open a remote file for writing
	remoteFile, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %v", remoteFilePath, err)
	}
	if err := session.Start("cat > " + remoteFilePath); err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}

	// open local file for reading
	// first check if file path is relative or absolute
	// if relative, then prepend the current working directory
	if !filepath.IsAbs(env.Spec.Kubernetes.KindConfig) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %v", err)
		}

		env.Spec.Kubernetes.KindConfig = filepath.Join(cwd, strings.TrimPrefix(env.Spec.Kubernetes.KindConfig, "./"))
	}

	localFile, err := os.Open(env.Spec.Kubernetes.KindConfig)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %v", env.Spec.Kubernetes.KindConfig, err)
	}

	// copy local file to remote file
	if _, err := io.Copy(remoteFile, localFile); err != nil {
		return fmt.Errorf("failed to copy local file %s to remote file %s: %v", env.Spec.Kubernetes.KindConfig, remoteFilePath, err)
	}

	// Close the writing pipe and wait for the session to finish
	remoteFile.Close() // nolint:errcheck, gosec
	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command to complete: %v", err)
	}

	return nil
}

// createKubeAdmConfig creates a kubeadm config file on the remote machine
// at /etc/kubernetes/kubeadm-config.yaml
func (p *Provisioner) createKubeAdmConfig(env v1alpha1.Environment) error {
	cachePath := filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
	// Define local and remote paths
	localFilePath := fmt.Sprintf("%s/kubeadm-config.yaml", cachePath)
	remoteFilePath := "/etc/kubernetes/kubeadm-config.yaml"
	tempRemotePath := "/tmp/kubeadm-config.yaml" // Temporary upload path

	// Ensure local directory exists
	if err := os.MkdirAll(cachePath, 0750); err != nil {
		return fmt.Errorf("failed to create local cache directory: %v", err)
	}

	// Create kubeadm config file locally
	var kubeadmConfigContent string

	kConfig, err := templates.NewKubeadmConfig(env)
	if err != nil {
		return fmt.Errorf("failed to create kubeadm config: %v", err)
	}

	kubeadmConfigContent, err = kConfig.GenerateKubeadmConfig()
	if err != nil {
		return fmt.Errorf("failed to generate kubeadm config: %v", err)
	}

	// Write kubeadm config to local file
	if err := os.WriteFile(localFilePath, []byte(kubeadmConfigContent), 0600); err != nil {
		return fmt.Errorf("failed to write kubeadm config to local file: %v", err)
	}

	// Copy the local file to the remote machine using SFTP
	if err := p.copyFileToRemoteSFTP(localFilePath, tempRemotePath); err != nil {
		return fmt.Errorf("failed to copy kubeadm config to remote host: %v", err)
	}

	// Move the temporary file to the final destination
	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	if err := session.Run("sudo mkdir -p /etc/kubernetes"); err != nil {
		session.Close() // nolint:errcheck, gosec
		return fmt.Errorf("failed to create directory on remote host: %v", err)
	}
	session.Close() // nolint:errcheck, gosec

	// Move the temporary file to the final destination
	session, err = p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	if err := session.Run(fmt.Sprintf("sudo mv %s %s", tempRemotePath, remoteFilePath)); err != nil {
		session.Close() // nolint:errcheck, gosec
		return fmt.Errorf("failed to move kubeadm config to final destination: %v", err)
	}
	session.Close() // nolint:errcheck, gosec

	return nil
}

// copyFileToRemoteSFTP copies a file from local to a remote server using SFTP.
func (p *Provisioner) copyFileToRemoteSFTP(localPath, remotePath string) error {
	// Open an SSH session
	client, err := sftp.NewClient(p.Client)
	if err != nil {
		return fmt.Errorf("failed to start SFTP session: %v", err)
	}
	defer client.Close() // nolint:errcheck, gosec

	// Open local file
	localFile, err := os.Open(localPath) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close() // nolint:errcheck, gosec

	// Open remote file for writing
	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close() // nolint:errcheck, gosec

	// Copy local file to remote file
	if _, err := remoteFile.ReadFrom(localFile); err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func addScriptHeader(tpl *bytes.Buffer) error {
	// Add shebang to the script
	shebang := template.Must(template.New("shebang").Parse(Shebang))
	if err := shebang.Execute(tpl, nil); err != nil {
		return fmt.Errorf("failed to add shebang to the script: %v", err)
	}
	// Add common functions to the script
	commonFunctions := template.Must(template.New("common-functions").Parse(templates.CommonFunctions))
	if err := commonFunctions.Execute(tpl, nil); err != nil {
		return fmt.Errorf("failed to add common functions to the script: %v", err)
	}
	return nil
}

// createSshClient creates a ssh client, and retries if it fails to connect
func connectOrDie(keyPath, userName, hostUrl string) (*ssh.Client, error) {
	var client *ssh.Client
	var err error
	key, err := os.ReadFile(keyPath) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %v", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
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
		client, err = ssh.Dial("tcp", hostUrl+":22", sshConfig)
		if err == nil {
			return client, nil // Connection succeeded, return the client.
		}
		connectionFailed = true
		// Sleep for a brief moment before retrying.
		// You can adjust the duration based on your requirements.
		time.Sleep(1 * time.Second)
	}

	if connectionFailed {
		return nil, fmt.Errorf("failed to connect to %s after 10 retries, giving up", hostUrl)
	}

	return client, nil
}
