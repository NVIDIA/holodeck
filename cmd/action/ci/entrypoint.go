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
	"io"
	"os"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

func entrypoint(log *logger.FunLogger) error {
	log.Info("Running Entrypoint function")

	configFile := os.Getenv("INPUT_HOLODECK_CONFIG")
	if configFile == "" {
		log.Error(fmt.Errorf("config file not provided"))
		os.Exit(1)
	}

	// Get INPUT_AWS_SSH_KEY and write it to a file
	sshKey := os.Getenv("INPUT_AWS_SSH_KEY")
	if sshKey == "" {
		log.Error(fmt.Errorf("ssh key not provided"))
		os.Exit(1)
	}

	err := os.WriteFile(sshKeyFile, []byte(sshKey), 0600)
	if err != nil {
		log.Error(fmt.Errorf("error writing ssh key to file: %s", err))
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

	// Read the config file
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}

	// If no containerruntime is specified, default to none
	if cfg.Spec.ContainerRuntime.Name == "" {
		cfg.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeNone
	}

	// Set env name
	sha := os.Getenv("GITHUB_SHA")
	// short sha
	if len(sha) > 8 {
		sha = sha[:8]
	}
	if sha == "" {
		log.Error(fmt.Errorf("github sha not provided"))
		os.Exit(1)
	}
	cfg.SetName(sha)

	client, err := aws.New(log, cfg, cacheFile)
	if err != nil {
		return err
	}

	err = client.Create()
	if err != nil {
		return err
	}

	// Read cache after creating the environment
	cache, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cacheFile)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	var hostUrl string

	log.Info("Provisioning \u2699")

	for _, p := range cache.Status.Properties {
		if p.Name == aws.PublicDnsName {
			hostUrl = p.Value
			break
		}
	}

	p, err := provisioner.New(log, cfg.Spec.Auth.PrivateKey, cfg.Spec.Auth.Username, hostUrl)
	if err != nil {
		return err
	}

	if err = p.Run(cfg); err != nil {
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	if cfg.Spec.Kubernetes.Install {
		err = getKubeConfig(log, &cfg, hostUrl)
		if err != nil {
			return fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	return nil
}

// getKubeConfig downloads the kubeconfig file from the remote host
func getKubeConfig(log *logger.FunLogger, cfg *v1alpha1.Environment, hostUrl string) error {
	remoteFilePath := "/home/ubuntu/.kube/config"

	// Create a new ssh session
	p, err := provisioner.New(log, cfg.Spec.Auth.PrivateKey, cfg.Spec.Auth.Username, hostUrl)
	if err != nil {
		return err
	}

	session, err := p.Client.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}
	defer session.Close()

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
	localFile, err := os.Create(kubeconfig)
	if err != nil {
		return fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

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

	log.Info(fmt.Sprintf("Kubeconfig saved to %s\n", kubeconfig))

	return nil
}
