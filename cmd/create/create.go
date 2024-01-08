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
	"log"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"

	"github.com/sirupsen/logrus"
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
	logger *logrus.Logger
}

// NewCommand constructs a validate command with the specified logger
func NewCommand(logger *logrus.Logger) *cli.Command {
	c := command{
		logger: logger,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	opts := options{}

	// Create the 'validate' command
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
				fmt.Printf("failed to read config file: %v\n", err)
				return err
			}

			// set cache path
			if opts.cachePath == "" {
				opts.cachePath = filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
			}
			opts.cachefile = filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")

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
		err := createAWS(opts)
		if err != nil {
			return fmt.Errorf("failed to create AWS infra: %v", err)
		}
		// Read cache after creating the environment
		opts.cache, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.cachefile)
		if err != nil {
			return fmt.Errorf("failed to read cache file: %v", err)
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		opts.cache = opts.cfg
	}

	if opts.provision {
		err := runProvision(opts)
		if err != nil {
			return fmt.Errorf("failed to provision: %v", err)
		}
	}

	return nil
}

func runProvision(opts *options) error {
	var hostUrl string
	if opts.cfg.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range opts.cfg.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		hostUrl = opts.cfg.Spec.Instance.HostUrl
	}

	p, err := provisioner.New(opts.cfg.Spec.Auth.PrivateKey, hostUrl)
	if err != nil {
		return err
	}

	if err = p.Run(opts.cfg); err != nil {
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	// Download kubeconfig
	if opts.cfg.Spec.Kubernetes.Install {
		if opts.cfg.Spec.Kubernetes.KubeConfig == "" {
			// and
			if opts.kubeconfig == "" {
				fmt.Printf("kubeconfig is not set, use default kubeconfig path: %s\n", filepath.Join(opts.cachePath, "kubeconfig"))
				// if kubeconfig is not set, use set to current directory as default
				// first get current directory
				pwd := os.Getenv("PWD")
				opts.kubeconfig = filepath.Join(pwd, "kubeconfig")
			}
			opts.cfg.Spec.Kubernetes.KubeConfig = opts.kubeconfig
		}

		// Create a session
		session, err := p.Client.NewSession()
		if err != nil {
			fmt.Printf("Failed to create session: %v\n", err)
			return err
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
		// Create a new file on the local system to save the downloaded content
		localFile, err := os.Create(opts.kubeconfig)
		if err != nil {
			return fmt.Errorf("error creating local file: %v", err)
		}
		defer localFile.Close()

		// Set up pipes for stdin, stdout, and stderr
		session.Stdout = localFile
		session.Stderr = os.Stderr

		// Run the SCP command to download the remote file
		remoteFilePath := "/home/ubuntu/.kube/config"
		err = session.Run("scp -f " + remoteFilePath)
		if err != nil {
			return fmt.Errorf("error running SCP command: %v", err)
		}

		fmt.Println("KubeConfig downloaded successfully.")
	}

	return nil
}

func createAWS(opts *options) error {
	cachefile := filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")
	client, err := aws.New(opts.cfg, cachefile)
	if err != nil {
		return err
	}

	err = client.Create()
	if err != nil {
		return err
	}

	return nil
}
