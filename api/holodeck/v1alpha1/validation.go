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

package v1alpha1

import (
	"fmt"
)

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
