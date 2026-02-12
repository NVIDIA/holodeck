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

package provisioner

import (
	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// BuildComponentsStatus creates a ComponentsStatus from the environment spec.
// This captures what was requested for provisioning (source, version, git refs)
// so the CLI can display provenance information.
func BuildComponentsStatus(env v1alpha1.Environment) *v1alpha1.ComponentsStatus {
	cs := &v1alpha1.ComponentsStatus{}
	hasComponents := false

	// NVIDIA Driver
	// Note: multi-source fields (Source, Package, Runfile, Git) are added in
	// Phase 1 (feat/issue-567-driver-sources). Until that merges, we only
	// track the legacy Branch/Version package fields.
	if env.Spec.NVIDIADriver.Install {
		hasComponents = true
		d := env.Spec.NVIDIADriver
		prov := &v1alpha1.ComponentProvenance{
			Source:  "package",
			Version: d.Version,
			Branch:  d.Branch,
		}
		cs.Driver = prov
	}

	// Container Runtime
	// Note: multi-source fields (Source, Package, Git, Latest) are added in
	// Phase 2 (feat/issue-567-runtime-sources). Until that merges, we only
	// track the legacy Name/Version package fields.
	if env.Spec.ContainerRuntime.Install {
		hasComponents = true
		cr := env.Spec.ContainerRuntime
		prov := &v1alpha1.ComponentProvenance{
			Source:  "package",
			Version: cr.Version,
		}
		cs.Runtime = prov
	}

	// NVIDIA Container Toolkit
	if env.Spec.NVIDIAContainerToolkit.Install {
		hasComponents = true
		nct := env.Spec.NVIDIAContainerToolkit
		prov := &v1alpha1.ComponentProvenance{
			Source: "package",
		}
		if string(nct.Source) != "" {
			prov.Source = string(nct.Source)
		}

		switch prov.Source {
		case "package":
			if nct.Package != nil {
				prov.Version = nct.Package.Version
			} else if nct.Version != "" {
				prov.Version = nct.Version
			}
		case "git":
			if nct.Git != nil {
				prov.Repo = nct.Git.Repo
				prov.Ref = nct.Git.Ref
			}
		case "latest":
			if nct.Latest != nil {
				prov.Branch = nct.Latest.Track
				prov.Repo = nct.Latest.Repo
			}
		}
		cs.Toolkit = prov
	}

	// Kubernetes
	if env.Spec.Kubernetes.Install {
		hasComponents = true
		k := env.Spec.Kubernetes
		prov := &v1alpha1.ComponentProvenance{
			Source: "release",
		}
		if string(k.Source) != "" {
			prov.Source = string(k.Source)
		}

		switch prov.Source {
		case "release":
			if k.Release != nil {
				prov.Version = k.Release.Version
			} else if k.KubernetesVersion != "" {
				prov.Version = k.KubernetesVersion
			}
		case "git":
			if k.Git != nil {
				prov.Repo = k.Git.Repo
				prov.Ref = k.Git.Ref
			}
		case "latest":
			if k.Latest != nil {
				prov.Branch = k.Latest.Track
				prov.Repo = k.Latest.Repo
			}
		}
		cs.Kubernetes = prov
	}

	if !hasComponents {
		return nil
	}
	return cs
}
