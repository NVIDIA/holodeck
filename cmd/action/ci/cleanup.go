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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cleanup(log *logger.FunLogger) error {
	log.Info("Running Cleanup function")

	// Read the config file
	configFile := os.Getenv("INPUT_HOLODECK_CONFIG")
	if configFile == "" {
		log.Error(fmt.Errorf("config file not provided"))
		os.Exit(1)
	}
	configFile = "/github/workspace/" + configFile
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}

	// Set env name
	setCfgName(&cfg)

	// check if cache exists
	if _, err := os.Stat(cacheFile); err != nil {
		fmt.Printf("Error reading cache file: %s\n", err)
		fmt.Printf("Cache file %s does not exist\n", cacheFile)
		os.Exit(1)
	}

	provider, err := newProvider(log, &cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %v", err)
	}

	if err := provider.Delete(); err != nil {
		log.Error(err)
		log.Exit(1)
	}

	// Delete the cache kubeconfig and ssh key
	// if kubeconfig exists, delete it
	if _, err := os.Stat(kubeconfig); err == nil {
		if err := os.Remove(kubeconfig); err != nil {
			log.Error(fmt.Errorf("error deleting kubeconfig: %s", err))
		}
	}

	if _, err := os.Stat(sshKeyFile); err == nil {
		if err := os.Remove(sshKeyFile); err != nil {
			log.Error(fmt.Errorf("error deleting ssh key: %s", err))
		}
	}

	log.Info("Successfully deleted environment %s\n", cfg.Name)

	return nil
}

func isTerminated(log *logger.FunLogger) (bool, error) {
	log.Info("Checking for Terminated condition")

	// Read the config file
	configFile := os.Getenv("INPUT_HOLODECK_CONFIG")
	if configFile == "" {
		log.Error(fmt.Errorf("config file not provided"))
		os.Exit(1)
	}
	configFile = "/github/workspace/" + configFile
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](configFile)
	if err != nil {
		return false, fmt.Errorf("error reading config file: %s", err)
	}

	provider, err := newProvider(log, &cfg)
	if err != nil {
		return false, fmt.Errorf("failed to create provider: %v", err)
	}

	status, err := provider.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %v", err)
	}

	for _, s := range status {
		if s.Type == v1alpha1.ConditionTerminated {
			return s.Status == metav1.ConditionTrue, nil
		}
	}

	return false, nil
}
