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

	"golang.org/x/crypto/ssh"

	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

const Shebang = `#! /usr/bin/env bash
set -xe
`

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

func (p *Provisioner) Run(env v1alpha1.Environment) error {
	dependencies := NewDependencies(env)

	// kind-config
	// Create kind config file if it is provided
	if env.Spec.Kubernetes.KubernetesInstaller == "kind" && env.Spec.Kubernetes.KindConfig != "" {
		if err := p.createKindConfig(env); err != nil {
			return fmt.Errorf("failed to create kind config file: %v", err)
		}
	}

	if env.Spec.Kubernetes.KubernetesInstaller == "kubeadm" {
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
		// Reset the connection, this step is needed to make sure some configuration changes take effect
		// e.g after installing docker, the user needs to be added to the docker group
		if err := p.resetConnection(); err != nil {
			return fmt.Errorf("failed to reset connection: %v", err)
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
		defer writer.Close()
		_, err := io.Copy(os.Stdout, reader)
		if err != nil {
			log.Fatalf("Failed to copy from reader: %v", err)
		}
	}()
	defer session.Close()

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
	remoteFilePath := "$HOME/kind.yaml"

	// Create a session
	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

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
	remoteFile.Close()
	session.Wait() // nolint:errcheck
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
	key, err := os.ReadFile(keyPath)
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
	for i := 0; i < 20; i++ {
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
