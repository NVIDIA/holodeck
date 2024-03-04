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
	"k8s.io/klog/v2"
)

func cleanup(log *logger.FunLogger) error {
	klog.Info("Running Cleanup function")

	configFile := os.Getenv("INPUT_HOLODECK_CONFIG")
	if configFile == "" {
		log.Error(fmt.Errorf("config file not provided"))
		os.Exit(1)
	}

	// Read the config file
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}

	client, err := aws.New(log, cfg, cacheFile)
	if err != nil {
		log.Error(err)
		log.Exit(1)
	}

	// check if cache exists
	if _, err := os.Stat(cacheFile); err != nil {
		fmt.Printf("Error reading cache file: %s\n", err)
		fmt.Printf("Cache file %s does not exist\n", cacheFile)
		os.Exit(1)
	}

	if err := client.Delete(); err != nil {
		log.Error(err)
		log.Exit(1)
	}

	// Delete the cache kubeconfig and ssh key
	if err := os.Remove(kubeconfig); err != nil {
		log.Error(fmt.Errorf("error deleting kubeconfig: %s", err))
	}

	if err := os.Remove(sshKeyFile); err != nil {
		log.Error(fmt.Errorf("error deleting ssh key: %s", err))
	}

	if err := os.RemoveAll(cachedir); err != nil {
		log.Error(fmt.Errorf("error deleting cache directory: %s", err))
	}

	log.Info("Successfully deleted environment %s\n", cfg.Name)

	return nil
}
