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

package utils

import (
	"fmt"
	"io"
	"os"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

// GetKubeConfig downloads the kubeconfig file from the remote host
func GetKubeConfig(log *logger.FunLogger, cfg *v1alpha1.Environment, hostUrl string, dest string) error {
	remoteFilePath := "${HOME}/.kube/config"

	// Create a new ssh session
	p, err := provisioner.New(log, cfg.Spec.PrivateKey, cfg.Spec.Username, hostUrl)
	if err != nil {
		return err
	}

	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}
	defer session.Close() // nolint:errcheck, gosec

	// Set up a pipe to receive the remote file content
	remoteFile, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating remote file pipe: %v", err)
	}

	// Start the remote command to read the file content
	err = session.Start(fmt.Sprintf("/usr/bin/cat  %s", remoteFilePath))
	if err != nil {
		return fmt.Errorf("error starting remote command: %v", err)
	}

	// Create a new file on the local system to save the downloaded content
	localFile, err := os.Create(dest) // nolint:gosec
	if err != nil {
		return fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close() // nolint:errcheck, gosec

	// Copy the remote file content to the local file
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("error copying remote file to local: %v", err)
	}

	// Wait for the remote command to finish
	err = session.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for remote command: %v", err)
	}

	log.Info(fmt.Sprintf("Kubeconfig saved to %s\n", dest))

	return nil
}
