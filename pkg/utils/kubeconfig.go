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

	"sigs.k8s.io/yaml"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

// kubeConfig is a minimal representation for server URL rewriting.
type kubeConfig struct {
	APIVersion     string                   `json:"apiVersion"`
	Kind           string                   `json:"kind"`
	Clusters       []kubeConfigClusterEntry `json:"clusters"`
	Contexts       []any                    `json:"contexts"`
	CurrentContext string                   `json:"current-context"`
	Users          []any                    `json:"users"`
}

type kubeConfigClusterEntry struct {
	Name    string            `json:"name"`
	Cluster kubeConfigCluster `json:"cluster"`
}

type kubeConfigCluster struct {
	Server                   string `json:"server"`
	CertificateAuthorityData string `json:"certificate-authority-data,omitempty"`
}

// RewriteKubeConfigServer rewrites the server URL in a kubeconfig file.
// If serverURL is empty, this is a no-op.
func RewriteKubeConfigServer(path string, serverURL string) error {
	if serverURL == "" {
		return nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is caller-provided kubeconfig
	if err != nil {
		return fmt.Errorf("reading kubeconfig: %w", err)
	}

	var cfg kubeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing kubeconfig: %w", err)
	}

	for i := range cfg.Clusters {
		cfg.Clusters[i].Cluster.Server = serverURL
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling kubeconfig: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("writing kubeconfig: %w", err)
	}

	return nil
}

// GetKubeConfig downloads the kubeconfig file from the remote host
func GetKubeConfig(log *logger.FunLogger, cfg *v1alpha1.Environment, hostUrl string, dest string, desiredServerURL string) error {
	remoteFilePath := "${HOME}/.kube/config"

	// Create a new ssh session
	p, err := provisioner.New(log, cfg.Spec.PrivateKey, cfg.Spec.Username, hostUrl)
	if err != nil {
		return err
	}
	defer func() { _ = p.Client.Close() }()

	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Set up a pipe to receive the remote file content
	remoteFile, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating remote file pipe: %w", err)
	}

	// Start the remote command to read the file content
	err = session.Start(fmt.Sprintf("/usr/bin/cat  %s", remoteFilePath))
	if err != nil {
		return fmt.Errorf("error starting remote command: %w", err)
	}

	// Create a new file on the local system to save the downloaded content
	localFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) //nolint:gosec // G304 — dest is a caller-provided kubeconfig path
	if err != nil {
		return fmt.Errorf("error creating local file: %w", err)
	}
	defer func() { _ = localFile.Close() }()

	// Copy the remote file content to the local file
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("error copying remote file to local: %w", err)
	}

	// Wait for the remote command to finish
	err = session.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for remote command: %w", err)
	}

	log.Info(fmt.Sprintf("Kubeconfig saved to %s\n", dest))

	if desiredServerURL != "" {
		if err := RewriteKubeConfigServer(dest, desiredServerURL); err != nil {
			return fmt.Errorf("failed to rewrite kubeconfig server URL: %w", err)
		}
		log.Info(fmt.Sprintf("Kubeconfig server URL rewritten to %s\n", desiredServerURL))
	}

	return nil
}
