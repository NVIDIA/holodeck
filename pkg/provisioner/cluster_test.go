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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestNewClusterProvisioner(t *testing.T) {
	log := logger.NewLogger()
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Cluster: &v1alpha1.ClusterSpec{
				Region: "us-west-2",
				ControlPlane: v1alpha1.ControlPlaneSpec{
					Count: 3,
				},
			},
		},
	}

	cp := NewClusterProvisioner(log, "/path/to/key", "ubuntu", env)

	assert.NotNil(t, cp)
	assert.Equal(t, "/path/to/key", cp.KeyPath)
	assert.Equal(t, "ubuntu", cp.UserName)
	assert.Equal(t, env, cp.Environment)
}

func TestClusterProvisioner_isHAEnabled(t *testing.T) {
	log := logger.NewLogger()

	tests := []struct {
		name     string
		env      *v1alpha1.Environment
		expected bool
	}{
		{
			name: "HA enabled",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						HighAvailability: &v1alpha1.HAConfig{
							Enabled: true,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "HA disabled",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						HighAvailability: &v1alpha1.HAConfig{
							Enabled: false,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "No HA config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{},
				},
			},
			expected: false,
		},
		{
			name: "No cluster config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "", "", tt.env)
			assert.Equal(t, tt.expected, cp.isHAEnabled())
		})
	}
}

func TestClusterProvisioner_isControlPlaneDedicated(t *testing.T) {
	log := logger.NewLogger()

	tests := []struct {
		name     string
		env      *v1alpha1.Environment
		expected bool
	}{
		{
			name: "Dedicated true",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						ControlPlane: v1alpha1.ControlPlaneSpec{
							Dedicated: true,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Dedicated false",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						ControlPlane: v1alpha1.ControlPlaneSpec{
							Dedicated: false,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "No cluster config",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "", "", tt.env)
			assert.Equal(t, tt.expected, cp.isControlPlaneDedicated())
		})
	}
}

func TestClusterProvisioner_getControlPlaneLabels(t *testing.T) {
	log := logger.NewLogger()

	tests := []struct {
		name     string
		env      *v1alpha1.Environment
		expected map[string]string
	}{
		{
			name: "Default labels only",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						ControlPlane: v1alpha1.ControlPlaneSpec{},
					},
				},
			},
			expected: map[string]string{
				"nvidia.com/holodeck.role": "control-plane",
			},
		},
		{
			name: "With custom labels",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						ControlPlane: v1alpha1.ControlPlaneSpec{
							Labels: map[string]string{
								"environment": "production",
								"tier":        "control",
							},
						},
					},
				},
			},
			expected: map[string]string{
				"nvidia.com/holodeck.role": "control-plane",
				"environment":              "production",
				"tier":                     "control",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "", "", tt.env)
			labels := cp.getControlPlaneLabels()
			assert.Equal(t, tt.expected, labels)
		})
	}
}

func TestClusterProvisioner_getWorkerLabels(t *testing.T) {
	log := logger.NewLogger()

	tests := []struct {
		name     string
		env      *v1alpha1.Environment
		expected map[string]string
	}{
		{
			name: "Default labels only (no workers)",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{},
				},
			},
			expected: map[string]string{
				"nvidia.com/holodeck.role": "worker",
			},
		},
		{
			name: "Default labels only (workers without labels)",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						Workers: &v1alpha1.WorkerPoolSpec{
							Count: 2,
						},
					},
				},
			},
			expected: map[string]string{
				"nvidia.com/holodeck.role": "worker",
			},
		},
		{
			name: "With custom labels",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{
						Workers: &v1alpha1.WorkerPoolSpec{
							Count: 2,
							Labels: map[string]string{
								"gpu":         "true",
								"environment": "production",
							},
						},
					},
				},
			},
			expected: map[string]string{
				"nvidia.com/holodeck.role": "worker",
				"gpu":                      "true",
				"environment":              "production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "", "", tt.env)
			labels := cp.getWorkerLabels()
			assert.Equal(t, tt.expected, labels)
		})
	}
}

func TestClusterProvisioner_determineControlPlaneEndpoint(t *testing.T) {
	log := logger.NewLogger()

	tests := []struct {
		name     string
		env      *v1alpha1.Environment
		firstCP  NodeInfo
		expected string
	}{
		{
			name: "Use load balancer DNS when available",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{},
				},
				Status: v1alpha1.EnvironmentStatus{
					Cluster: &v1alpha1.ClusterStatus{
						LoadBalancerDNS: "my-lb.elb.amazonaws.com",
					},
				},
			},
			firstCP: NodeInfo{
				PrivateIP: "10.0.0.1",
			},
			expected: "my-lb.elb.amazonaws.com",
		},
		{
			name: "Fall back to first CP private IP",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{},
				},
				Status: v1alpha1.EnvironmentStatus{},
			},
			firstCP: NodeInfo{
				PrivateIP: "10.0.0.1",
			},
			expected: "10.0.0.1",
		},
		{
			name: "No cluster status",
			env: &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Cluster: &v1alpha1.ClusterSpec{},
				},
			},
			firstCP: NodeInfo{
				PrivateIP: "10.0.0.2",
			},
			expected: "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "", "", tt.env)
			endpoint := cp.determineControlPlaneEndpoint(tt.firstCP)
			assert.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestClusterProvisioner_ProvisionCluster_Validation(t *testing.T) {
	log := logger.NewLogger()
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Cluster: &v1alpha1.ClusterSpec{
				Region: "us-west-2",
			},
		},
	}

	tests := []struct {
		name    string
		nodes   []NodeInfo
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Empty nodes list",
			nodes:   []NodeInfo{},
			wantErr: true,
			errMsg:  "no nodes to provision",
		},
		{
			name: "No control-plane nodes",
			nodes: []NodeInfo{
				{Name: "worker-0", Role: "worker", PublicIP: "1.2.3.4", PrivateIP: "10.0.0.1"},
			},
			wantErr: true,
			errMsg:  "at least one control-plane node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewClusterProvisioner(log, "/fake/key", "ubuntu", env)
			err := cp.ProvisionCluster(tt.nodes)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				// Note: This would fail in actual execution due to SSH connection
				// We're only testing validation here
				assert.NoError(t, err)
			}
		})
	}
}

func TestNodeInfo(t *testing.T) {
	node := NodeInfo{
		Name:      "cp-0",
		PublicIP:  "1.2.3.4",
		PrivateIP: "10.0.0.1",
		Role:      "control-plane",
	}

	assert.Equal(t, "cp-0", node.Name)
	assert.Equal(t, "1.2.3.4", node.PublicIP)
	assert.Equal(t, "10.0.0.1", node.PrivateIP)
	assert.Equal(t, "control-plane", node.Role)
}

func TestClusterHealth(t *testing.T) {
	health := ClusterHealth{
		Healthy:         true,
		TotalNodes:      3,
		ReadyNodes:      3,
		ControlPlanes:   1,
		Workers:         2,
		APIServerStatus: "Running",
		Message:         "Cluster healthy: 3/3 nodes ready",
		Nodes: []NodeHealth{
			{Name: "cp-0", Role: "control-plane", Ready: true, Status: "Ready", Version: "v1.31.0"},
			{Name: "worker-0", Role: "worker", Ready: true, Status: "Ready", Version: "v1.31.0"},
			{Name: "worker-1", Role: "worker", Ready: true, Status: "Ready", Version: "v1.31.0"},
		},
	}

	assert.True(t, health.Healthy)
	assert.Equal(t, 3, health.TotalNodes)
	assert.Equal(t, 3, health.ReadyNodes)
	assert.Equal(t, 1, health.ControlPlanes)
	assert.Equal(t, 2, health.Workers)
	assert.Equal(t, "Running", health.APIServerStatus)
	assert.Len(t, health.Nodes, 3)
}

func TestNodeHealth(t *testing.T) {
	node := NodeHealth{
		Name:       "worker-0",
		Role:       "worker",
		Ready:      true,
		Status:     "Ready",
		Version:    "v1.31.0",
		InternalIP: "10.0.0.2",
	}

	assert.Equal(t, "worker-0", node.Name)
	assert.Equal(t, "worker", node.Role)
	assert.True(t, node.Ready)
	assert.Equal(t, "Ready", node.Status)
	assert.Equal(t, "v1.31.0", node.Version)
	assert.Equal(t, "10.0.0.2", node.InternalIP)
}
