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
)

const (
	cachedir   = "/github/workspace/.cache"
	cacheFile  = "/github/workspace/.cache/holodeck.yaml"
	kubeconfig = "/github/workspace/kubeconfig"
	sshKeyFile = "/github/workspace/.cache/key.pem"
	// Default EC2 instance UserName for ubuntu AMI's
	username = "ubuntu"
)

func Run(log *logger.FunLogger) error {
	log.Info("Holodeck Settting up test environment")

	if _, err := os.Stat(cachedir); err == nil {
		if err := cleanup(log); err != nil {
			return err
		}
	} else {
		if err := entrypoint(log); err != nil {
			if err := cleanup(log); err != nil {
				return err
			}
			return err
		}
	}

	log.Check("Holodeck completed successfully")

	return nil
}

func setCfgName(cfg *v1alpha1.Environment) {
	sha := os.Getenv("GITHUB_SHA")
	// short sha
	if len(sha) > 8 {
		sha = sha[:8]
	}
	cfg.Name = fmt.Sprintf("ci-%s", sha)
}
