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

type Kubernetes struct {
	Version               string
	KubeletReleaseVersion string
	Arch                  string
	CniPluginsVersion     string
	CalicoVersion         string
	CrictlVersion         string
	K8sEndpointHost       string
	K8sFeatureGates       string
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

	// Add shebang to the script
	shebang := template.Must(template.New("shebang").Parse(Shebang))
	if err = shebang.Execute(&p.tpl, nil); err != nil {
		return nil, fmt.Errorf("failed to add shebang to the script: %v", err)
	}

	return p, nil
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
	fmt.Printf("Connecting to %s\n", hostUrl)
	connectionFailed := false
	for i := 0; i < 10; i++ {
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
	// 1. nv-driver
	// 2. container-runtime
	// 3. container-toolkit
	// 4. kubernetes
	var containerRuntime string

	// 1. nv-driver
	if env.Spec.NVDriver.Install {
		nvDriverTemplate := template.Must(template.New("nv-driver").Parse(templates.NvDriver))
		err := nvDriverTemplate.Execute(&p.tpl, &NvDriver{})
		if err != nil {
			return fmt.Errorf("failed to execute nv-driver template: %v", err)
		}
	}

	// 2. container-runtime
	if env.Spec.ContainerRuntime.Install {
		switch env.Spec.ContainerRuntime.Name {
		case "docker":
			containerRuntime = "docker"
			dockerTemplate := template.Must(template.New("docker").Parse(templates.Docker))
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
	}

	// 3. container-toolkit
	// We need to install container-toolkit after container-runtime or skip it
	if env.Spec.NVContainerToolKit.Install && env.Spec.ContainerRuntime.Install {
		containerToolkitTemplate := template.Must(template.New("container-toolkit").Parse(templates.ContainerToolkit))
		err := containerToolkitTemplate.Execute(&p.tpl, &ContainerToolkit{ContainerRuntime: containerRuntime})
		if err != nil {
			return fmt.Errorf("failed to execute container-toolkit template: %v", err)
		}
	}

	// 4. kubernetes
	// Set opinionated defaults if not set
	if env.Spec.Kubernetes.Install {
		kubernetesTemplate := template.Must(template.New("kubernetes").Parse(templates.Kubernetes))
		kubernetes := &Kubernetes{
			Version: env.Spec.Kubernetes.KubernetesVersion,
		}
		// check if env.Spec.Kubernetes.KubernetesVersion is in the format of vX.Y.Z
		// if not, set the default version
		if !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v") {
			fmt.Printf("Kubernetes version %s is not in the format of vX.Y.Z, setting default version v1.27.9\n", env.Spec.Kubernetes.KubernetesVersion)
			kubernetes.Version = "v1.27.9"
		}
		if env.Spec.Kubernetes.KubeletReleaseVersion != "" {
			kubernetes.KubeletReleaseVersion = env.Spec.Kubernetes.KubeletReleaseVersion
		} else {
			kubernetes.KubeletReleaseVersion = "v0.16.2"
		}
		if env.Spec.Kubernetes.Arch != "" {
			kubernetes.Arch = env.Spec.Kubernetes.Arch
		} else {
			kubernetes.Arch = "amd64"
		}
		if env.Spec.Kubernetes.CniPluginsVersion != "" {
			kubernetes.CniPluginsVersion = env.Spec.Kubernetes.CniPluginsVersion
		} else {
			kubernetes.CniPluginsVersion = "v0.8.7"
		}
		if env.Spec.Kubernetes.CalicoVersion != "" {
			kubernetes.CalicoVersion = env.Spec.Kubernetes.CalicoVersion
		} else {
			kubernetes.CalicoVersion = "v3.27.0"
		}
		if env.Spec.Kubernetes.CrictlVersion != "" {
			kubernetes.CrictlVersion = env.Spec.Kubernetes.CrictlVersion
		} else {
			kubernetes.CrictlVersion = "v1.22.0"
		}
		if env.Spec.Kubernetes.K8sEndpointHost != "" {
			kubernetes.K8sEndpointHost = env.Spec.Kubernetes.K8sEndpointHost
		} else {
			kubernetes.K8sEndpointHost = p.HostUrl
		}
		if len(env.Spec.Kubernetes.K8sFeatureGates) > 0 {
			// convert []string to string
			kubernetes.K8sFeatureGates = strings.Join(env.Spec.Kubernetes.K8sFeatureGates, ",")
		}

		err := kubernetesTemplate.Execute(&p.tpl, kubernetes)
		if err != nil {
			return fmt.Errorf("failed to execute kubernetes template: %v", err)
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
