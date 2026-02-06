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

package common

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
)

// GetHostURL resolves the SSH-reachable host URL for an environment.
// If nodeName is set, it looks for that specific node.
// If preferControlPlane is true and no nodeName is set, it prefers a control-plane node.
// Falls back to the first available node, then single-node properties.
func GetHostURL(env *v1alpha1.Environment, nodeName string, preferControlPlane bool) (string, error) {
	// For multinode clusters, find the appropriate node
	if env.Spec.Cluster != nil && env.Status.Cluster != nil && len(env.Status.Cluster.Nodes) > 0 {
		if nodeName != "" {
			for _, node := range env.Status.Cluster.Nodes {
				if node.Name == nodeName {
					return node.PublicIP, nil
				}
			}
			return "", fmt.Errorf("node %q not found in cluster", nodeName)
		}

		if preferControlPlane {
			for _, node := range env.Status.Cluster.Nodes {
				if node.Role == "control-plane" {
					return node.PublicIP, nil
				}
			}
		}

		// Fallback to first node
		return env.Status.Cluster.Nodes[0].PublicIP, nil
	}

	// Single node - get from properties
	switch env.Spec.Provider {
	case v1alpha1.ProviderAWS:
		for _, p := range env.Status.Properties {
			if p.Name == aws.PublicDnsName {
				return p.Value, nil
			}
		}
	case v1alpha1.ProviderSSH:
		return env.Spec.HostUrl, nil
	}

	return "", fmt.Errorf("unable to determine host URL")
}

const (
	// sshMaxRetries is the number of SSH connection attempts before giving up.
	sshMaxRetries = 3
	// sshRetryDelay is the delay between SSH connection retry attempts.
	sshRetryDelay = 2 * time.Second
)

// ConnectSSH establishes an SSH connection with retries.
//
// Host key verification is disabled because server host keys are generated
// at instance boot time and there is no trusted channel to distribute them
// to the client beforehand. The env file's privateKey/publicKey fields are
// SSH *authentication* keys (client-to-server), not server host keys.
func ConnectSSH(log *logger.FunLogger, keyPath, userName, hostUrl string) (*ssh.Client, error) {
	key, err := os.ReadFile(keyPath) //nolint:gosec // keyPath is from trusted env config
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %v", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// Server host keys are generated at instance boot with no trusted
		// distribution channel; disable verification (CWE-322 accepted risk).
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         30 * time.Second,
	}

	var client *ssh.Client
	for i := 0; i < sshMaxRetries; i++ {
		client, err = ssh.Dial("tcp", hostUrl+":22", config)
		if err == nil {
			return client, nil
		}
		log.Warning("Connection attempt %d failed: %v", i+1, err)
		time.Sleep(sshRetryDelay)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %v", sshMaxRetries, err)
}
