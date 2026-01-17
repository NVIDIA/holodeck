/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package v1alpha1

import (
	"fmt"
)

// Validate validates the NVIDIAContainerToolkit configuration.
func (nct *NVIDIAContainerToolkit) Validate() error {
	if !nct.Install {
		return nil
	}

	source := nct.Source
	if source == "" {
		source = CTKSourcePackage
	}

	switch source {
	case CTKSourcePackage:
		// Package source is always valid
		if nct.Package != nil && nct.Package.Channel != "" {
			if nct.Package.Channel != "stable" && nct.Package.Channel != "experimental" {
				return fmt.Errorf(
					"invalid CTK package channel: %s (must be 'stable' or 'experimental')",
					nct.Package.Channel,
				)
			}
		}
		return nil

	case CTKSourceGit:
		if nct.Git == nil {
			return fmt.Errorf("CTK git source requires 'git' configuration")
		}
		if nct.Git.Ref == "" {
			return fmt.Errorf("CTK git source requires 'ref' to be specified")
		}
		return nil

	case CTKSourceLatest:
		// Latest source is valid with or without explicit config
		return nil

	default:
		return fmt.Errorf("unknown CTK source: %s", source)
	}
}

// Validate validates the Kubernetes configuration.
func (k *Kubernetes) Validate() error {
	if !k.Install {
		return nil
	}

	source := k.Source
	if source == "" {
		source = K8sSourceRelease
	}

	installer := k.KubernetesInstaller
	if installer == "" {
		installer = "kubeadm"
	}

	switch source {
	case K8sSourceRelease:
		// Release source is valid; version can come from Release.Version or
		// legacy KubernetesVersion field
		return nil

	case K8sSourceGit:
		// MicroK8s does not support git source
		if installer == "microk8s" {
			return fmt.Errorf(
				"Kubernetes git source is not supported with microk8s installer; " +
					"use kubeadm or kind instead",
			)
		}
		if k.Git == nil {
			return fmt.Errorf("Kubernetes git source requires 'git' configuration")
		}
		if k.Git.Ref == "" {
			return fmt.Errorf("Kubernetes git source requires 'ref' to be specified")
		}
		return nil

	case K8sSourceLatest:
		// MicroK8s does not support latest source
		if installer == "microk8s" {
			return fmt.Errorf(
				"Kubernetes latest source is not supported with microk8s installer; " +
					"use kubeadm or kind instead",
			)
		}
		// Latest source is valid with or without explicit config
		return nil

	default:
		return fmt.Errorf("unknown Kubernetes source: %s", source)
	}
}
