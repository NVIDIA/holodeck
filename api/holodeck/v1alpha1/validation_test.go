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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterSpec_Validate(t *testing.T) {
	rootVolumeSize := int32(64)
	smallVolumeSize := int32(10)

	tests := []struct {
		name    string
		cluster *ClusterSpec
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Nil cluster - always valid",
			cluster: nil,
			wantErr: false,
		},
		{
			name: "Valid single control-plane cluster",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count:        1,
					InstanceType: "m5.xlarge",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid 3-node HA cluster",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count:        3,
					InstanceType: "m5.xlarge",
				},
				Workers: &WorkerPoolSpec{
					Count:        2,
					InstanceType: "g4dn.xlarge",
				},
				HighAvailability: &HAConfig{
					Enabled:      true,
					EtcdTopology: EtcdTopologyStacked,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid 5-node HA cluster",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count:            5,
					InstanceType:     "m5.xlarge",
					Dedicated:        true,
					RootVolumeSizeGB: &rootVolumeSize,
				},
				Workers: &WorkerPoolSpec{
					Count:            10,
					InstanceType:     "g4dn.xlarge",
					RootVolumeSizeGB: &rootVolumeSize,
				},
				HighAvailability: &HAConfig{
					Enabled:          true,
					EtcdTopology:     EtcdTopologyStacked,
					LoadBalancerType: "nlb",
				},
			},
			wantErr: false,
		},
		{
			name: "Control-plane only cluster (no workers)",
			cluster: &ClusterSpec{
				Region: "us-east-1",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "Missing region",
			cluster: &ClusterSpec{
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
			},
			wantErr: true,
			errMsg:  "region is required",
		},
		{
			name: "Control plane count zero",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 0,
				},
			},
			wantErr: true,
			errMsg:  "at least 1",
		},
		{
			name: "Control plane count too high",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 9,
				},
			},
			wantErr: true,
			errMsg:  "at most 7",
		},
		{
			name: "Even control plane count (bad for etcd quorum)",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 2,
				},
			},
			wantErr: true,
			errMsg:  "odd number",
		},
		{
			name: "Even control plane count of 4",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 4,
				},
			},
			wantErr: true,
			errMsg:  "odd number",
		},
		{
			name: "HA enabled with only 1 control-plane node",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
				HighAvailability: &HAConfig{
					Enabled: true,
				},
			},
			wantErr: true,
			errMsg:  "at least 3 control-plane nodes",
		},
		{
			name: "HA disabled - single node is fine",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
				HighAvailability: &HAConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "External etcd topology (not yet supported)",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 3,
				},
				HighAvailability: &HAConfig{
					Enabled:      true,
					EtcdTopology: EtcdTopologyExternal,
				},
			},
			wantErr: true,
			errMsg:  "not yet supported",
		},
		{
			name: "Invalid etcd topology",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 3,
				},
				HighAvailability: &HAConfig{
					Enabled:      true,
					EtcdTopology: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid etcd topology",
		},
		{
			name: "Invalid load balancer type",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 3,
				},
				HighAvailability: &HAConfig{
					Enabled:          true,
					LoadBalancerType: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid load balancer type",
		},
		{
			name: "Control plane root volume too small",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count:            1,
					RootVolumeSizeGB: &smallVolumeSize,
				},
			},
			wantErr: true,
			errMsg:  "at least 20GB",
		},
		{
			name: "Worker root volume too small",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
				Workers: &WorkerPoolSpec{
					Count:            2,
					RootVolumeSizeGB: &smallVolumeSize,
				},
			},
			wantErr: true,
			errMsg:  "at least 20GB",
		},
		{
			name: "Negative worker count",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
				Workers: &WorkerPoolSpec{
					Count: -1,
				},
			},
			wantErr: true,
			errMsg:  "cannot be negative",
		},
		{
			name: "Zero workers is valid",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
				},
				Workers: &WorkerPoolSpec{
					Count: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "With OS field",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
					OS:    "ubuntu-22.04",
				},
				Workers: &WorkerPoolSpec{
					Count: 2,
					OS:    "ubuntu-22.04",
				},
			},
			wantErr: false,
		},
		{
			name: "With custom labels",
			cluster: &ClusterSpec{
				Region: "us-west-2",
				ControlPlane: ControlPlaneSpec{
					Count: 1,
					Labels: map[string]string{
						"team": "gpu-infra",
					},
				},
				Workers: &WorkerPoolSpec{
					Count: 2,
					Labels: map[string]string{
						"gpu": "true",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestControlPlaneSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cp      ControlPlaneSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid single node",
			cp: ControlPlaneSpec{
				Count: 1,
			},
			wantErr: false,
		},
		{
			name: "Valid 3 nodes",
			cp: ControlPlaneSpec{
				Count: 3,
			},
			wantErr: false,
		},
		{
			name: "Valid 5 nodes",
			cp: ControlPlaneSpec{
				Count: 5,
			},
			wantErr: false,
		},
		{
			name: "Valid 7 nodes (max)",
			cp: ControlPlaneSpec{
				Count: 7,
			},
			wantErr: false,
		},
		{
			name: "Zero nodes",
			cp: ControlPlaneSpec{
				Count: 0,
			},
			wantErr: true,
			errMsg:  "at least 1",
		},
		{
			name: "Too many nodes",
			cp: ControlPlaneSpec{
				Count: 9,
			},
			wantErr: true,
			errMsg:  "at most 7",
		},
		{
			name: "Even count of 2",
			cp: ControlPlaneSpec{
				Count: 2,
			},
			wantErr: true,
			errMsg:  "odd number",
		},
		{
			name: "Even count of 6",
			cp: ControlPlaneSpec{
				Count: 6,
			},
			wantErr: true,
			errMsg:  "odd number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cp.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerPoolSpec_Validate(t *testing.T) {
	smallVolume := int32(10)
	validVolume := int32(64)

	tests := []struct {
		name    string
		wp      WorkerPoolSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid with 1 worker",
			wp: WorkerPoolSpec{
				Count: 1,
			},
			wantErr: false,
		},
		{
			name: "Valid with many workers",
			wp: WorkerPoolSpec{
				Count: 100,
			},
			wantErr: false,
		},
		{
			name: "Zero workers is valid",
			wp: WorkerPoolSpec{
				Count: 0,
			},
			wantErr: false,
		},
		{
			name: "Negative workers",
			wp: WorkerPoolSpec{
				Count: -1,
			},
			wantErr: true,
			errMsg:  "cannot be negative",
		},
		{
			name: "Small root volume",
			wp: WorkerPoolSpec{
				Count:            1,
				RootVolumeSizeGB: &smallVolume,
			},
			wantErr: true,
			errMsg:  "at least 20GB",
		},
		{
			name: "Valid root volume",
			wp: WorkerPoolSpec{
				Count:            1,
				RootVolumeSizeGB: &validVolume,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wp.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHAConfig_Validate(t *testing.T) {
	tests := []struct {
		name              string
		ha                HAConfig
		controlPlaneCount int32
		wantErr           bool
		errMsg            string
	}{
		{
			name: "HA disabled - always valid",
			ha: HAConfig{
				Enabled: false,
			},
			controlPlaneCount: 1,
			wantErr:           false,
		},
		{
			name: "Valid HA with stacked etcd",
			ha: HAConfig{
				Enabled:      true,
				EtcdTopology: EtcdTopologyStacked,
			},
			controlPlaneCount: 3,
			wantErr:           false,
		},
		{
			name: "Valid HA with NLB",
			ha: HAConfig{
				Enabled:          true,
				LoadBalancerType: "nlb",
			},
			controlPlaneCount: 3,
			wantErr:           false,
		},
		{
			name: "Valid HA with ALB",
			ha: HAConfig{
				Enabled:          true,
				LoadBalancerType: "alb",
			},
			controlPlaneCount: 3,
			wantErr:           false,
		},
		{
			name: "External etcd not supported",
			ha: HAConfig{
				Enabled:      true,
				EtcdTopology: EtcdTopologyExternal,
			},
			controlPlaneCount: 3,
			wantErr:           true,
			errMsg:            "not yet supported",
		},
		{
			name: "Invalid etcd topology",
			ha: HAConfig{
				Enabled:      true,
				EtcdTopology: "invalid",
			},
			controlPlaneCount: 3,
			wantErr:           true,
			errMsg:            "invalid etcd topology",
		},
		{
			name: "Invalid load balancer type",
			ha: HAConfig{
				Enabled:          true,
				LoadBalancerType: "elb",
			},
			controlPlaneCount: 3,
			wantErr:           true,
			errMsg:            "invalid load balancer type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ha.Validate(tt.controlPlaneCount)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
