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
	"regexp"
)

var k8sLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._\-/]*[a-zA-Z0-9])?$`)

func validateLabels(labels map[string]string) error {
	for k, v := range labels {
		if !k8sLabelPattern.MatchString(k) {
			return fmt.Errorf("invalid label key %q: contains disallowed characters", k)
		}
		if v != "" && !k8sLabelPattern.MatchString(v) {
			return fmt.Errorf("invalid label value %q for key %q: contains disallowed characters", v, k)
		}
	}
	return nil
}

// Validate validates the ClusterSpec configuration.
func (c *ClusterSpec) Validate() error {
	if c == nil {
		return nil
	}

	// Validate region is specified
	if c.Region == "" {
		return fmt.Errorf("cluster region is required")
	}

	// Validate control plane
	if err := c.ControlPlane.Validate(); err != nil {
		return fmt.Errorf("control plane validation failed: %w", err)
	}

	// Validate workers if specified
	if c.Workers != nil {
		if err := c.Workers.Validate(); err != nil {
			return fmt.Errorf("workers validation failed: %w", err)
		}
	}

	// Validate labels for shell-injection safety
	if err := validateLabels(c.ControlPlane.Labels); err != nil {
		return fmt.Errorf("control-plane labels: %w", err)
	}
	if c.Workers != nil {
		if err := validateLabels(c.Workers.Labels); err != nil {
			return fmt.Errorf("worker labels: %w", err)
		}
	}

	// Validate HA configuration
	if c.HighAvailability != nil {
		if err := c.HighAvailability.Validate(c.ControlPlane.Count); err != nil {
			return fmt.Errorf("high availability validation failed: %w", err)
		}
	}

	// Additional cross-field validations
	if c.HighAvailability != nil && c.HighAvailability.Enabled {
		// HA requires at least 3 control-plane nodes for quorum
		if c.ControlPlane.Count < 3 {
			return fmt.Errorf(
				"high availability requires at least 3 control-plane nodes, got %d",
				c.ControlPlane.Count,
			)
		}
	}

	return nil
}

// Validate validates the ControlPlaneSpec configuration.
func (cp *ControlPlaneSpec) Validate() error {
	// Validate count bounds
	if cp.Count < 1 {
		return fmt.Errorf("control plane count must be at least 1, got %d", cp.Count)
	}
	if cp.Count > 7 {
		return fmt.Errorf("control plane count must be at most 7, got %d", cp.Count)
	}

	// For etcd quorum, control-plane count should be odd
	if cp.Count > 1 && cp.Count%2 == 0 {
		return fmt.Errorf(
			"control plane count should be an odd number (1, 3, 5, 7) for etcd quorum, got %d",
			cp.Count,
		)
	}

	// Validate that either OS or Image is specified (but not required if defaults
	// work)
	// Both can be empty if defaults are acceptable

	// Validate root volume size if specified
	if cp.RootVolumeSizeGB != nil && *cp.RootVolumeSizeGB < 20 {
		return fmt.Errorf(
			"control plane root volume size must be at least 20GB, got %d",
			*cp.RootVolumeSizeGB,
		)
	}

	return nil
}

// Validate validates the WorkerPoolSpec configuration.
func (wp *WorkerPoolSpec) Validate() error {
	// Count can be 0 (control-plane only cluster)
	if wp.Count < 0 {
		return fmt.Errorf("worker count cannot be negative, got %d", wp.Count)
	}

	// Validate root volume size if specified
	if wp.RootVolumeSizeGB != nil && *wp.RootVolumeSizeGB < 20 {
		return fmt.Errorf(
			"worker root volume size must be at least 20GB, got %d",
			*wp.RootVolumeSizeGB,
		)
	}

	return nil
}

// Validate validates the HAConfig configuration.
func (ha *HAConfig) Validate(controlPlaneCount int32) error {
	if !ha.Enabled {
		return nil
	}

	// Validate etcd topology
	if ha.EtcdTopology != "" &&
		ha.EtcdTopology != EtcdTopologyStacked &&
		ha.EtcdTopology != EtcdTopologyExternal {
		return fmt.Errorf(
			"invalid etcd topology: %s (must be 'stacked' or 'external')",
			ha.EtcdTopology,
		)
	}

	// External etcd requires more careful consideration
	if ha.EtcdTopology == EtcdTopologyExternal {
		// External etcd is an advanced configuration
		// For now, just warn that it's not fully implemented
		// In the future, this would require additional etcd node configuration
		return fmt.Errorf(
			"external etcd topology is not yet supported; use 'stacked' topology",
		)
	}

	// Validate load balancer type
	if ha.LoadBalancerType != "" &&
		ha.LoadBalancerType != "nlb" &&
		ha.LoadBalancerType != "alb" {
		return fmt.Errorf(
			"invalid load balancer type: %s (must be 'nlb' or 'alb')",
			ha.LoadBalancerType,
		)
	}

	return nil
}

// Validate validates the NVIDIADriver configuration.
func (d *NVIDIADriver) Validate() error {
	if !d.Install {
		return nil
	}

	source := d.Source
	if source == "" {
		source = DriverSourcePackage
	}

	switch source {
	case DriverSourcePackage:
		// Package source is always valid; branch/version are optional
		return nil

	case DriverSourceRunfile:
		if d.Runfile == nil {
			return fmt.Errorf("driver runfile source requires 'runfile' configuration")
		}
		if d.Runfile.URL == "" {
			return fmt.Errorf("driver runfile source requires 'url' to be specified")
		}
		return nil

	case DriverSourceGit:
		if d.Git == nil {
			return fmt.Errorf("driver git source requires 'git' configuration")
		}
		if d.Git.Ref == "" {
			return fmt.Errorf("driver git source requires 'ref' to be specified")
		}
		return nil

	default:
		return fmt.Errorf("unknown driver source: %s", source)
	}
}

// Validate validates the ContainerRuntime configuration.
func (cr *ContainerRuntime) Validate() error {
	if !cr.Install {
		return nil
	}

	source := cr.Source
	if source == "" {
		source = RuntimeSourcePackage
	}

	switch source {
	case RuntimeSourcePackage:
		// Package source is always valid; version is optional
		return nil

	case RuntimeSourceGit:
		if cr.Git == nil {
			return fmt.Errorf("container runtime git source requires 'git' configuration")
		}
		if cr.Git.Ref == "" {
			return fmt.Errorf("container runtime git source requires 'ref' to be specified")
		}
		return nil

	case RuntimeSourceLatest:
		// Latest source is valid with or without explicit config
		return nil

	default:
		return fmt.Errorf("unknown container runtime source: %s", source)
	}
}

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
