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

package ci

import (
	"fmt"
	"os"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provider/vsphere"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/pkg/utils"
)

func entrypoint(log *logger.FunLogger) error {
	log.Info("Running Entrypoint function")

	configFile := os.Getenv("INPUT_HOLODECK_CONFIG")
	if configFile == "" {
		log.Error(fmt.Errorf("config file not provided"))
		os.Exit(1)
	}
	configFile = "/github/workspace/" + configFile

	// Read the config file
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}
	// If no containerruntime is specified, default to none
	if cfg.Spec.ContainerRuntime.Name == "" {
		cfg.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeNone
	}

	// Read Inputs
	// INPUT_* vars are optional since v0.2 of the action
	// Users can set the variables on self hosted runners.
	// Get INPUT_AWS_SSH_KEY to set AWS_SSH_KEY
	sshKey := os.Getenv("INPUT_AWS_SSH_KEY")
	if sshKey != "" {
		err := os.Setenv("AWS_SSH_KEY", sshKey)
		if err != nil {
			return fmt.Errorf("failed to set AWS_SSH_KEY: %v", err)
		}
	}
	// Map INPUT_AWS_ACCESS_KEY_ID and INPUT_AWS_SECRET_ACCESS_KEY
	// to AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	accessKeyID := os.Getenv("INPUT_AWS_ACCESS_KEY_ID")
	if accessKeyID != "" {
		err := os.Setenv("AWS_ACCESS_KEY_ID", accessKeyID)
		if err != nil {
			return fmt.Errorf("failed to set AWS_ACCESS_KEY_ID: %v", err)
		}
	}
	secretAccessKey := os.Getenv("INPUT_AWS_SECRET_ACCESS_KEY")
	if secretAccessKey != "" {
		err := os.Setenv("AWS_SECRET_ACCESS_KEY", secretAccessKey)
		if err != nil {
			return fmt.Errorf("failed to set AWS_SECRET_ACCESS_KEY: %v", err)
		}
	}

	provider, err := newProvider(log, cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %v", err)
	}

	err = provider.Create()
	if err != nil {
		return err
	}

	// Read cache after creating the environment
	cache, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	// Get the host url
	var hostUrl string
	var username string
	if cfg.Spec.Provider == v1alpha1.ProviderAWS {
		username = "ubuntu"
		for _, p := range cache.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
	} else if cfg.Spec.Provider == v1alpha1.ProviderVSphere {
		username = "nvidia"
		for _, p := range cache.Status.Properties {
			if p.Name == vsphere.IpAddress {
				hostUrl = p.Value
				break
			}
		}
	}

	// Run the provisioner
	p, err := provisioner.New(log, sshKeyFile, username, hostUrl)
	if err != nil {
		return err
	}
	defer p.Client.Close()

	log.Info("Provisioning \u2699")
	if err = p.Run(cfg); err != nil {
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	if cfg.Spec.Kubernetes.Install {
		err = utils.GetKubeConfig(log, &cfg, hostUrl, kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	return nil
}
