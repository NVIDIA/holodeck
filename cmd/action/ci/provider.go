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
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provider/vsphere"
)

func newProvider(log *logger.FunLogger, cfg v1alpha1.Environment) (provider.Provider, error) {
	var provider provider.Provider
	var err error

	switch cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		provider, err = newAwsProvider(log, cfg)
		if err != nil {
			return nil, err
		}
	case v1alpha1.ProviderVSphere:
		provider, err = newVsphereProvider(log, cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("provider %s not supported", cfg.Spec.Provider)
	}

	return provider, nil
}

func newAwsProvider(log *logger.FunLogger, cfg v1alpha1.Environment) (*aws.Provider, error) {
	// Get INPUT_AWS_SSH_KEY and write it to a file
	sshKey := os.Getenv("INPUT_AWS_SSH_KEY")
	if sshKey == "" {
		log.Error(fmt.Errorf("ssh key not provided"))
		os.Exit(1)
	}

	// Map INPUT_AWS_ACCESS_KEY_ID and INPUT_AWS_SECRET_ACCESS_KEY
	// to AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	accessKeyID := os.Getenv("INPUT_AWS_ACCESS_KEY_ID")
	if accessKeyID == "" {
		log.Error(fmt.Errorf("aws access key id not provided"))
		os.Exit(1)
	}

	secretAccessKey := os.Getenv("INPUT_AWS_SECRET_ACCESS_KEY")
	if secretAccessKey == "" {
		log.Error(fmt.Errorf("aws secret access key not provided"))
		os.Exit(1)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", accessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", secretAccessKey)

	// Create cachedir directory
	if _, err := os.Stat(cachedir); os.IsNotExist(err) {
		err := os.Mkdir(cachedir, 0755)
		if err != nil {
			log.Error(fmt.Errorf("error creating cache directory: %s", err))
			os.Exit(1)
		}
	}

	err := os.WriteFile(sshKeyFile, []byte(sshKey), 0600)
	if err != nil {
		log.Error(fmt.Errorf("error writing ssh key to file: %s", err))
		os.Exit(1)
	}

	// Set auth.PrivateKey
	cfg.Spec.Auth.PrivateKey = sshKeyFile
	cfg.Spec.Auth.Username = username

	// Set env name
	setCfgName(&cfg)

	a, err := aws.New(log, cfg, cacheFile)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func newVsphereProvider(log *logger.FunLogger, cfg v1alpha1.Environment) (*vsphere.Provider, error) {
	// Create cachedir directory
	if _, err := os.Stat(cachedir); os.IsNotExist(err) {
		err := os.Mkdir(cachedir, 0755)
		if err != nil {
			log.Error(fmt.Errorf("error creating cache directory: %s", err))
			os.Exit(1)
		}
	}
	// Get INPUT_HOLODECK_SSH_KEY and write it to a file
	sshKey := os.Getenv("INPUT_HOLODECK_SSH_KEY")
	if sshKey == "" {
		log.Error(fmt.Errorf("ssh key not provided"))
		os.Exit(1)
	}
	err := os.WriteFile(sshKeyFile, []byte(sshKey), 0600)
	if err != nil {
		log.Error(fmt.Errorf("error writing ssh key to file: %s", err))
		os.Exit(1)
	}

	// Map INPUT_HOLODECK_VCENTER_USERNAME and INPUT_HOLODECK_VCENTER_PASSWORD
	// to HOLODECK_VCENTER_USERNAME and HOLODECK_VCENTER_PASSWORD
	os.Setenv("HOLODECK_VCENTER_USERNAME", os.Getenv("INPUT_HOLODECK_VCENTER_USERNAME"))
	os.Setenv("HOLODECK_VCENTER_PASSWORD", os.Getenv("INPUT_HOLODECK_VCENTER_PASSWORD"))

	// Set env name
	setCfgName(&cfg)

	v, err := vsphere.New(log, cfg, cacheFile)
	if err != nil {
		return nil, err
	}

	return v, nil
}
