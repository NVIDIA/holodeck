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
)

func newProvider(log *logger.FunLogger, cfg *v1alpha1.Environment) (provider.Provider, error) {
	var provider provider.Provider
	var err error

	switch cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		provider, err = newAwsProvider(log, cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("provider %s not supported", cfg.Spec.Provider)
	}

	return provider, nil
}

func newAwsProvider(log *logger.FunLogger, cfg *v1alpha1.Environment) (*aws.Provider, error) {
	// Create cachedir directory
	if _, err := os.Stat(cachedir); os.IsNotExist(err) {
		err := os.Mkdir(cachedir, 0750)
		if err != nil {
			log.Error(fmt.Errorf("error creating cache directory: %w", err))
			return nil, err
		}
	}

	// Set env name
	setCfgName(cfg)

	a, err := aws.New(log, *cfg, cacheFile)
	if err != nil {
		return nil, err
	}

	return a, nil
}

// look for file holodeck_ssh_key in GITHUB_WORKSPACE/holodeck_ssh_key
// if file not found, look for env var envKey
// if env var not found, return error
func getSSHKeyFile(log *logger.FunLogger, envKey string) error {
	if _, err := os.Stat(holodeckSSHKeyFile); os.IsNotExist(err) {
		// look for env var set KEY
		envSshKey := os.Getenv(envKey)
		if envSshKey == "" {
			log.Error(fmt.Errorf("ssh key not provided"))
			return fmt.Errorf("ssh key not provided")
		}
		err := os.WriteFile(sshKeyFile, []byte(envSshKey), 0600) //nolint:gosec // G703: sshKeyFile is a hardcoded constant
		if err != nil {
			log.Error(fmt.Errorf("error writing ssh key to file: %w", err))
			return err
		}
	} else {
		// copy file to sshKeyFile
		err := os.Rename(holodeckSSHKeyFile, sshKeyFile)
		if err != nil {
			log.Error(fmt.Errorf("error copying ssh key file: %w", err))
			return err
		}
	}

	return nil
}
