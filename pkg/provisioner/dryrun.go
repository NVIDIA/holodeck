/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package provisioner

import (
	"fmt"
	"strings"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

// validateCTKConfig validates the NVIDIA Container Toolkit configuration.
func validateCTKConfig(log *logger.FunLogger, env v1alpha1.Environment) error {
	nct := env.Spec.NVIDIAContainerToolkit
	source := nct.Source
	if source == "" {
		source = v1alpha1.CTKSourcePackage
	}

	switch source {
	case v1alpha1.CTKSourcePackage:
		switch {
		case nct.Package != nil && nct.Package.Version != "":
			log.Info("CTK source: package (version: %s)", nct.Package.Version)
		case nct.Version != "":
			log.Info("CTK source: package (version: %s, legacy field)", nct.Version)
		default:
			log.Info("CTK source: package (latest)")
		}

	case v1alpha1.CTKSourceGit:
		if nct.Git == nil {
			return fmt.Errorf("CTK git source requires 'git' configuration")
		}
		if nct.Git.Ref == "" {
			return fmt.Errorf("CTK git source requires 'ref' to be specified")
		}
		repo := nct.Git.Repo
		if repo == "" {
			repo = "https://github.com/NVIDIA/nvidia-container-toolkit.git"
		}
		log.Info("CTK source: git (ref: %s, repo: %s)", nct.Git.Ref, repo)

	case v1alpha1.CTKSourceLatest:
		track := "main"
		repo := "https://github.com/NVIDIA/nvidia-container-toolkit.git"
		if nct.Latest != nil {
			if nct.Latest.Track != "" {
				track = nct.Latest.Track
			}
			if nct.Latest.Repo != "" {
				repo = nct.Latest.Repo
			}
		}
		log.Info("CTK source: latest (branch: %s, repo: %s)", track, repo)

	default:
		return fmt.Errorf("unknown CTK source: %s", source)
	}

	return nil
}

// Dryrun validates the environment configuration without making changes.
func Dryrun(log *logger.FunLogger, env v1alpha1.Environment) error {
	// Resolve dependencies from top to bottom
	cancel := log.Loading("Resolving dependencies \U0001F4E6\n")
	// Kubernetes -> Container Toolkit -> Container Runtime -> NVDriver
	if env.Spec.Kubernetes.Install && env.Spec.Kubernetes.KubernetesInstaller == "kubeadm" {
		// check if env.Spec.Kubernetes.KubernetesVersion is in the format of vX.Y.Z
		if env.Spec.Kubernetes.KubernetesVersion != "" {
			if !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v") {
				cancel(logger.ErrLoadingFailed)
				return fmt.Errorf("kubernetes version %s is not in the format of vX.Y.Z", env.Spec.Kubernetes.KubernetesVersion)
			}
		}
	}

	if env.Spec.ContainerRuntime.Install {
		if env.Spec.ContainerRuntime.Name == "" {
			log.Warning("No container runtime specified, will default to containerd")
		} else if env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeContainerd &&
			env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeCrio &&
			env.Spec.ContainerRuntime.Name != v1alpha1.ContainerRuntimeDocker {
			cancel(logger.ErrLoadingFailed)
			return fmt.Errorf("container runtime %s not supported", env.Spec.ContainerRuntime.Name)
		}
	}

	// Validate CTK configuration
	if env.Spec.NVIDIAContainerToolkit.Install {
		if err := validateCTKConfig(log, env); err != nil {
			cancel(logger.ErrLoadingFailed)
			return err
		}
	}

	cancel(nil)
	log.Wg.Wait()

	return nil
}
