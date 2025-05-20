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
	"os"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/pkg/utils"

	cli "github.com/urfave/cli/v2"
)

type options struct {
	provision  bool
	cachePath  string
	envFile    string
	kubeconfig string

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
	var provider provider.Provider
	var err error

	// Create instance manager and generate unique ID
	manager := instances.NewManager(m.log, opts.cachePath)
	instanceID := manager.GenerateInstanceID()
	opts.cachePath = manager.GetInstanceCacheFile(instanceID)

	// Add instance ID to environment metadata
	if opts.cfg.Labels == nil {
		opts.cfg.Labels = make(map[string]string)
	}
	opts.cfg.Labels[instances.InstanceLabelKey] = instanceID
	opts.cfg.Labels[instances.InstanceProvisionedLabelKey] = "false"

	switch opts.cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		if opts.cfg.Spec.Username == "" {
			// TODO (ArangoGutierrez): This should be based on the OS
			// Amazon Linux: ec2-user
			// Ubuntu: ubuntu
			// CentOS: centos
			// Debian: admin
			// RHEL: ec2-user
			// Fedora: ec2-user
			// SUSE: ec2-user
			opts.cfg.Spec.Username = "ubuntu"
		}
		provider, err = aws.New(m.log, opts.cfg, opts.cachePath)
		if err != nil {
			return err
		}

	case v1alpha1.ProviderSSH:
		if opts.cfg.Spec.Username == "" {
			opts.cfg.Spec.Username = os.Getenv("USER")
		}
		m.log.Info("SSH infrastructure \u2601")
		opts.cache = opts.cfg
	}

	err = provider.Create()
	if err != nil {
		return err
	}

	// Read cache after creating the environment
	opts.cache, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.cachePath)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	if opts.provision {
		err := runProvision(m.log, opts)
		if err != nil {
			return fmt.Errorf("failed to provision: %v", err)
		}
	}

	m.log.Info("Created instance %s", instanceID)
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
		hostUrl = opts.cfg.Spec.HostUrl
	}

	p, err := provisioner.New(log, opts.cfg.Spec.PrivateKey, opts.cfg.Spec.Username, hostUrl)
	if err != nil {
		return err
	}
	defer p.Client.Close() // nolint: errcheck

	// Copy cache status into the environment
	opts.cfg.Status = opts.cache.Status

	if err = p.Run(opts.cfg); err != nil {
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	// Set provisioning status to true after successful provisioning
	opts.cfg.Labels[instances.InstanceProvisionedLabelKey] = "true"
	data, err := jyaml.MarshalYAML(opts.cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %v", err)
	}
	if err := os.WriteFile(opts.cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to update cache file with provisioning status: %v", err)
	}

	// Download kubeconfig
	if opts.cfg.Spec.Kubernetes.Install && (opts.cfg.Spec.Kubernetes.KubeConfig != "" || opts.kubeconfig != "") {
		if opts.cfg.Spec.Kubernetes.KubernetesInstaller == "microk8s" || opts.cfg.Spec.Kubernetes.KubernetesInstaller == "kind" {
			log.Warning("kubeconfig retrieval is not supported for %s, skipping kubeconfig download", opts.cfg.Spec.Kubernetes.KubernetesInstaller)
			return nil
		}

		if err = utils.GetKubeConfig(log, &opts.cache, hostUrl, opts.kubeconfig); err != nil {
			return fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	return nil
}
