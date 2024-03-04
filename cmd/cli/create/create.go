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

package create

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	cli "github.com/urfave/cli/v2"
)

type options struct {
	provision  bool
	cachePath  string
	envFile    string
	kubeconfig string

	cachefile string

	cfg   v1alpha1.Environment
	cache v1alpha1.Environment
}

type command struct {
	log *logger.FunLogger
}

// NewCommand constructs the create command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	opts := options{}

	// Create the 'create' command
	create := cli.Command{
		Name:  "create",
		Usage: "create a test environment based on config file",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "provision",
				Aliases:     []string{"p"},
				Usage:       "Provision the environment",
				Destination: &opts.provision,
			},
			&cli.StringFlag{
				Name:        "kubeconfig",
				Aliases:     []string{"k"},
				Usage:       "Path create to the kubeconfig file",
				Destination: &opts.kubeconfig,
			},
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &opts.cachePath,
			},
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
				return fmt.Errorf("error reading config file: %s", err)
			}

			// set cache path
			if opts.cachePath == "" {
				opts.cachePath = filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
			}
			opts.cachefile = filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")

			// if no containerruntime is specified, default to none
			if opts.cfg.Spec.ContainerRuntime.Name == "" {
				opts.cfg.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeNone
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			return m.run(c, &opts)
		},
	}

	return &create
}

func (m command) run(c *cli.Context, opts *options) error {
	if opts.cfg.Spec.Provider == v1alpha1.ProviderAWS {
		// If no username is specified, default to ubuntu
		if opts.cfg.Spec.Auth.Username == "" {
			// TODO (ArangoGutierrez): This should be based on the OS
			// Amazon Linux: ec2-user
			// Ubuntu: ubuntu
			// CentOS: centos
			// Debian: admin
			// RHEL: ec2-user
			// Fedora: ec2-user
			// SUSE: ec2-user
			opts.cfg.Spec.Auth.Username = "ubuntu"
		}
		err := createAWS(m.log, opts)
		if err != nil {
			return fmt.Errorf("failed to create AWS infra: %v", err)
		}
		// Read cache after creating the environment
		opts.cache, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.cachefile)
		if err != nil {
			return fmt.Errorf("failed to read cache file: %v", err)
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		// If username is not provided, use the current user
		if opts.cfg.Spec.Username == "" {
			opts.cfg.Spec.Username = os.Getenv("USER")
		}
		m.log.Info("SSH infrastructure \u2601")
		opts.cache = opts.cfg
	}

	if opts.provision {
		err := runProvision(m.log, opts)
		if err != nil {
			return fmt.Errorf("failed to provision: %v", err)
		}
	}

	return nil
}

func runProvision(log *logger.FunLogger, opts *options) error {
	var hostUrl string

	log.Info("Provisioning \u2699")

	if opts.cfg.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range opts.cache.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		hostUrl = opts.cfg.Spec.Instance.HostUrl
	}

	p, err := provisioner.New(log, opts.cfg.Spec.Auth.PrivateKey, opts.cfg.Spec.Auth.Username, hostUrl)
	if err != nil {
		return err
	}

	if err = p.Run(opts.cfg); err != nil {
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	// Download kubeconfig
	if opts.cfg.Spec.Kubernetes.Install && (opts.cfg.Spec.Kubernetes.KubeConfig != "" || opts.kubeconfig != "") {
		if opts.cfg.Spec.Kubernetes.KubernetesInstaller == "microk8s" || opts.cfg.Spec.Kubernetes.KubernetesInstaller == "kind" {
			log.Warning("kubeconfig is not supported for %s, skipping kubeconfig download", opts.cfg.Spec.Kubernetes.KubernetesInstaller)
			return nil
		}

		if err = getKubeConfig(log, opts, hostUrl); err != nil {
			return fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	return nil
}

// getKubeConfig downloads the kubeconfig file from the remote host
func getKubeConfig(log *logger.FunLogger, opts *options, hostUrl string) error {
	remoteFilePath := "/home/ubuntu/.kube/config"
	if opts.cfg.Spec.Kubernetes.KubeConfig == "" {
		// and
		if opts.kubeconfig == "" {
			log.Warning("kubeconfig is not set, use default kubeconfig path: %s\n", filepath.Join(opts.cachePath, "kubeconfig"))
			// if kubeconfig is not set, use set to current directory as default
			// first get current directory
			pwd := os.Getenv("PWD")
			opts.kubeconfig = filepath.Join(pwd, "kubeconfig")
		} else {
			opts.cfg.Spec.Kubernetes.KubeConfig = opts.kubeconfig
		}
	}

	// Create a new ssh session
	p, err := provisioner.New(log, opts.cfg.Spec.Auth.PrivateKey, opts.cfg.Spec.Auth.Username, hostUrl)
	if err != nil {
		return err
	}

	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}
	defer session.Close()

	// Set up a pipe to receive the remote file content
	remoteFile, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating remote file pipe: %v", err)
	}

	// Start the remote command to read the file content
	err = session.Start(fmt.Sprintf("/usr/bin/cat  %s", remoteFilePath))
	if err != nil {
		return fmt.Errorf("error starting remote command: %v", err)
	}

	// Create a new file on the local system to save the downloaded content
	localFile, err := os.Create(opts.kubeconfig)
	if err != nil {
		return fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

	// Copy the remote file content to the local file
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("error copying remote file to local: %v", err)
	}

	// Wait for the remote command to finish
	err = session.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for remote command: %v", err)
	}

	log.Info(fmt.Sprintf("Kubeconfig saved to %s\n", opts.kubeconfig))

	return nil
}

func createAWS(log *logger.FunLogger, opts *options) error {
	log.Info("Creating AWS infrastructure \u2601")
	client, err := aws.New(log, opts.cfg, opts.cachefile)
	if err != nil {
		return err
	}

	err = client.Create()
	if err != nil {
		return err
	}

	return nil
}
