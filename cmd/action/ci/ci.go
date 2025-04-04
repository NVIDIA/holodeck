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
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

const (
	cachedir           = "/github/workspace/.cache"
	cacheFile          = "/github/workspace/.cache/holodeck.yaml"
	kubeconfig         = "/github/workspace/kubeconfig"
	sshKeyFile         = "/github/workspace/.cache/key"
	holodeckSSHKeyFile = "/github/workspace/holodeck_ssh_key"
)

func Run(log *logger.FunLogger) error {
	// Get GitHub Actions INPUT_* vars
	err := readInputs()
	if err != nil {
		return err
	}

	_, err = os.Stat(cachedir)
	if os.IsNotExist(err) {
		if err := entrypoint(log); err != nil {
			log.Error(err)
			if err := cleanup(log); err != nil {
				return err
			}
			return err
		}
		return nil
	}

	// Check if cache condition is Terminated
	if ok, err := isTerminated(log); ok {
		log.Info("Environment condition is Terminated no need to run Holodeck")
		return nil
	} else if err != nil {
		log.Warning("%s", err.Error())
	}
	if err := cleanup(log); err != nil {
		return err
	}

	return nil
}

// readInputs reads GitHub Actions Inputs
// INPUT_* vars are optional since v0.2 of the action
// Users can set the variables on self hosted runners.
func readInputs() error {
	// Get INPUT_AWS_SSH_KEY to set AWS_SSH_KEY
	awsSshKey := os.Getenv("INPUT_AWS_SSH_KEY")
	if awsSshKey != "" {
		err := os.Setenv("AWS_SSH_KEY", awsSshKey)
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

	// For vSphere
	vsphereSshKey := os.Getenv("INPUT_VSPHERE_SSH_KEY")
	if vsphereSshKey != "" {
		err := os.Setenv("VSPHERE_SSH_KEY", vsphereSshKey)
		if err != nil {
			return fmt.Errorf("failed to set VSPHERE_SSH_KEY: %v", err)
		}
	}
	// Map INPUT_VSPHERE_USERNAME and INPUT_VSPHERE_PASSWORD
	// to HOLODECK_VCENTER_USERNAME and HOLODECK_VCENTER_PASSWORD
	vsphereUsername := os.Getenv("INPUT_VSPHERE_USERNAME")
	if vsphereUsername != "" {
		err := os.Setenv("HOLODECK_VCENTER_USERNAME", vsphereUsername)
		if err != nil {
			return fmt.Errorf("failed to set HOLODECK_VCENTER_USERNAME: %v", err)
		}
	}
	vspherePassword := os.Getenv("INPUT_VSPHERE_PASSWORD")
	if vspherePassword != "" {
		err := os.Setenv("HOLODECK_VCENTER_PASSWORD", vspherePassword)
		if err != nil {
			return fmt.Errorf("failed to set HOLODECK_VCENTER_PASSWORD: %v", err)
		}
	}

	return nil
}

func setCfgName(cfg *v1alpha1.Environment) {
	sha := os.Getenv("GITHUB_SHA")
	attempt := os.Getenv("GITHUB_RUN_ATTEMPT")
	// short sha
	if len(sha) > 8 {
		sha = sha[:8]
	}
	// uid is unique for each run
	uid := generateUID()

	cfg.Name = fmt.Sprintf("ci%s-%s-%s", attempt, sha, uid)
}

func generateUID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)

	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("failed to generate secure random UID: %v", err)
	}

	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}

	return string(b)
}
