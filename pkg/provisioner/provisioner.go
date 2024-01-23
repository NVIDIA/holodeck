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
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

const Shebang = `#! /usr/bin/env bash
set -xe
`

type Provisioner struct {
	Client         *ssh.Client
	SessionManager *ssm.Client

	HostUrl string
	tpl     bytes.Buffer
}

type Containerd struct {
	Version string
}

type Crio struct {
	Version string
}

type Docker struct {
	Version string
}

type ContainerToolkit struct {
	ContainerRuntime string
}

type NvDriver struct {
	// Empty struct
	// Placeholder to enable type assertion
}

func New(keyPath, hostUrl string) (*Provisioner, error) {
	fmt.Printf("Connecting to %s\n", hostUrl)
	client, err := connectOrDie(keyPath, hostUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", hostUrl, err)
	}

	p := &Provisioner{
		Client:  client,
		HostUrl: hostUrl,
		tpl:     bytes.Buffer{},
	}

	// Add script header and common functions to the script
	if err := addScriptHeader(&p.tpl); err != nil {
		return nil, fmt.Errorf("failed to add shebang to the script: %v", err)
	}

	return p, nil
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

// resetConnection resets the ssh connection, and retries if it fails to connect
func (p *Provisioner) resetConnection(keyPath, hostUrl string) error {
	var err error

	// Close the current ssh connection
	if err := p.Client.Close(); err != nil {
		return fmt.Errorf("failed to close ssh client: %v", err)
	}

	// Create a new ssh connection
	p.Client, err = connectOrDie(keyPath, hostUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", p.HostUrl, err)
	}

	return nil
}

// createSshClient creates a ssh client, and retries if it fails to connect
func connectOrDie(keyPath, hostUrl string) (*ssh.Client, error) {
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
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	connectionFailed := false
	for i := 0; i < 20; i++ {
		client, err = ssh.Dial("tcp", hostUrl+":22", sshConfig)
		if err == nil {
			return client, nil // Connection succeeded, return the client.
		}
		fmt.Printf("Failed to connect to %s: %v\n", hostUrl, err)
		connectionFailed = true
		// Sleep for a brief moment before retrying.
		// You can adjust the duration based on your requirements.
		time.Sleep(1 * time.Second)
	}

	if connectionFailed {
		fmt.Printf("Failed to connect to %s after 10 retries, giving up\n", hostUrl)
		return nil, err
	}

	return client, nil
}

func (p *Provisioner) Run(env v1alpha1.Environment) error {
	// Read v1alpha1.Provisioner and execute the template based on the config
	// the logic order is:
	// 1. container-runtime
	// 2. container-toolkit
	// 3. nv-driver
	// 4. kubernetes
	// 5. kind-config (if kubernetes is installed via kind)
	var containerRuntime string

	// 1. container-runtime
	if env.Spec.ContainerRuntime.Install {
		switch env.Spec.ContainerRuntime.Name {
		case "docker":
			containerRuntime = "docker"
			dockerTemplate := template.Must(template.New("docker").Parse(templates.Docker))
			if env.Spec.ContainerRuntime.Version == "" {
				env.Spec.ContainerRuntime.Version = "latest"
			}
			err := dockerTemplate.Execute(&p.tpl, &Docker{Version: env.Spec.ContainerRuntime.Version})
			if err != nil {
				return fmt.Errorf("failed to execute docker template: %v", err)
			}
		case "crio":
			containerRuntime = "crio"
			crioTemplate := template.Must(template.New("crio").Parse(templates.Crio))
			err := crioTemplate.Execute(&p.tpl, &Crio{Version: env.Spec.ContainerRuntime.Version})
			if err != nil {
				return fmt.Errorf("failed to execute crio template: %v", err)
			}
		case "containerd":
			containerRuntime = "containerd"
			containerdTemplate := template.Must(template.New("containerd").Parse(templates.Containerd))
			err := containerdTemplate.Execute(&p.tpl, &Containerd{Version: env.Spec.ContainerRuntime.Version})
			if err != nil {
				return fmt.Errorf("failed to execute containerd template: %v", err)
			}
		default:
			fmt.Printf("Unknown container runtime %s\n", env.Spec.ContainerRuntime.Name)
			return nil
		}
	} else if env.Spec.Kubernetes.KubernetesInstaller == "kind" {
		// If kubernetes is installed via kind, we need to install docker
		// as the container runtime
		containerRuntime = "docker"
		dockerTemplate := template.Must(template.New("docker").Parse(templates.Docker))
		err := dockerTemplate.Execute(&p.tpl, &Docker{Version: "latest"})
		if err != nil {
			return fmt.Errorf("failed to execute docker template: %v", err)
		}

		// And since we want to use KIND non-root mode, we need to add the user
		// to the docker group so that the user can run docker commands without
		// sudo
		if err := p.provision(); err != nil {
			return fmt.Errorf("failed to provision: %v", err)
		}
		p.tpl.Reset()
		if err := addScriptHeader(&p.tpl); err != nil {
			return fmt.Errorf("failed to add shebang to the script: %v", err)
		}
		// close session to force docker group to take effect
		if err := p.resetConnection(env.Spec.Auth.PrivateKey, p.HostUrl); err != nil {
			return fmt.Errorf("failed to reset ssh connection: %v", err)
		}
	}

	// 2. container-toolkit
	// We need to install container-toolkit after container-runtime or skip it
	// We also need to install container-toolkit if kubernetes is installed
	// via kind
	if env.Spec.NVContainerToolKit.Install && env.Spec.ContainerRuntime.Install || env.Spec.Kubernetes.KubernetesInstaller == "kind" {
		containerToolkitTemplate := template.Must(template.New("container-toolkit").Parse(templates.ContainerToolkit))
		err := containerToolkitTemplate.Execute(&p.tpl, &ContainerToolkit{ContainerRuntime: containerRuntime})
		if err != nil {
			return fmt.Errorf("failed to execute container-toolkit template: %v", err)
		}
	}

	// 3. nv-driver
	// We need to install nv-driver if container-runtime if Kind is used
	if env.Spec.NVDriver.Install || env.Spec.Kubernetes.KubernetesInstaller == "kind" {
		nvDriverTemplate := template.Must(template.New("nv-driver").Parse(templates.NvDriver))
		err := nvDriverTemplate.Execute(&p.tpl, &NvDriver{})
		if err != nil {
			return fmt.Errorf("failed to execute nv-driver template: %v", err)
		}
	}

	// 4. kubernetes
	// Set opinionated defaults if not set
	if env.Spec.Kubernetes.Install {
		if env.Spec.Kubernetes.K8sEndpointHost == "" {
			env.Spec.Kubernetes.K8sEndpointHost = p.HostUrl
		}
		err := templates.ExecuteKubernetes(&p.tpl, env)
		if err != nil {
			return fmt.Errorf("failed to execute kubernetes template: %v", err)
		}
	}

	// 5. kind-config
	// Create kind config file if it is set
	if env.Spec.Kubernetes.KubernetesInstaller == "kind" && env.Spec.Kubernetes.KindConfig != "" {
		if err := p.createKindConfig(env); err != nil {
			return fmt.Errorf("failed to create kind config file: %v", err)
		}
	}

	// Provision the instance
	if err := p.provision(); err != nil {
		return fmt.Errorf("failed to provision: %v", err)
	}

	return nil
}

func (p *Provisioner) provision() error {
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
	session.Wait()
	return nil
}
