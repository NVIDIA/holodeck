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
	"os"

	"github.com/NVIDIA/holodeck/internal/logger"
)

const (
	cachedir   = "/github/workspace/.cache"
	cacheFile  = "/github/workspace/.cache/holodeck.yaml"
	kubeconfig = "/github/workspace/kubeconfig"
	sshKeyFile = "/github/workspace/.cache/key"
)

func Run(log *logger.FunLogger) error {
	log.Info("Running Holodeck function")
	// Check if .cache folder exists in the /github/workspace directory and if it does, call cleanup function
	// If it doesn't, call entrypoint function
	// If the entrypoint function returns an error, call cleanup function
	if _, err := os.Stat(cachedir); err == nil {
		if err := cleanup(log); err != nil {
			return err
		}
	} else {
		if err := entrypoint(log); err != nil {
			log.Error(err)
			if err := cleanup(log); err != nil {
				return err
			}
			return err
		}
	}

	log.Check("Holodeck completed successfully")

	return nil
}
