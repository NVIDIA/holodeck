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

func TestNVIDIADriver_Validate(t *testing.T) {
	tests := []struct {
		name    string
		driver  NVIDIADriver
		wantErr bool
		errMsg  string
	}{
		{
			name: "Install disabled - always valid",
			driver: NVIDIADriver{
				Install: false,
			},
			wantErr: false,
		},
		{
			name: "Package source - default (no config)",
			driver: NVIDIADriver{
				Install: true,
			},
			wantErr: false,
		},
		{
			name: "Package source - explicit with branch",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourcePackage,
				Package: &DriverPackageSpec{
					Branch: "560",
				},
			},
			wantErr: false,
		},
		{
			name: "Package source - explicit with version",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourcePackage,
				Package: &DriverPackageSpec{
					Version: "560.35.03",
				},
			},
			wantErr: false,
		},
		{
			name: "Package source - legacy fields",
			driver: NVIDIADriver{
				Install: true,
				Branch:  "550",
				Version: "550.90.07",
			},
			wantErr: false,
		},
		{
			name: "Runfile source - valid",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceRunfile,
				Runfile: &DriverRunfileSpec{
					URL: "https://download.nvidia.com/driver.run",
				},
			},
			wantErr: false,
		},
		{
			name: "Runfile source - with checksum",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceRunfile,
				Runfile: &DriverRunfileSpec{
					URL:      "https://download.nvidia.com/driver.run",
					Checksum: "sha256:abc123",
				},
			},
			wantErr: false,
		},
		{
			name: "Runfile source - missing config",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceRunfile,
			},
			wantErr: true,
			errMsg:  "runfile source requires",
		},
		{
			name: "Runfile source - missing URL",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceRunfile,
				Runfile: &DriverRunfileSpec{},
			},
			wantErr: true,
			errMsg:  "url",
		},
		{
			name: "Git source - valid",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceGit,
				Git: &DriverGitSpec{
					Ref: "560.35.03",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - with custom repo",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceGit,
				Git: &DriverGitSpec{
					Repo: "https://github.com/myorg/open-gpu-kernel-modules.git",
					Ref:  "main",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - missing config",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceGit,
			},
			wantErr: true,
			errMsg:  "git source requires",
		},
		{
			name: "Git source - missing ref",
			driver: NVIDIADriver{
				Install: true,
				Source:  DriverSourceGit,
				Git:     &DriverGitSpec{},
			},
			wantErr: true,
			errMsg:  "ref",
		},
		{
			name: "Unknown source",
			driver: NVIDIADriver{
				Install: true,
				Source:  "unknown",
			},
			wantErr: true,
			errMsg:  "unknown driver source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.Validate()
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

func TestContainerRuntime_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cr      ContainerRuntime
		wantErr bool
		errMsg  string
	}{
		{
			name: "Install disabled - always valid",
			cr: ContainerRuntime{
				Install: false,
			},
			wantErr: false,
		},
		{
			name: "Package source - default (no config)",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
			},
			wantErr: false,
		},
		{
			name: "Package source - explicit with version",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourcePackage,
				Package: &RuntimePackageSpec{
					Version: "1.7.23",
				},
			},
			wantErr: false,
		},
		{
			name: "Package source - legacy version field",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeDocker,
				Version: "24.0.0",
			},
			wantErr: false,
		},
		{
			name: "Git source - valid",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourceGit,
				Git: &RuntimeGitSpec{
					Ref: "v1.7.23",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - with custom repo",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeCrio,
				Source:  RuntimeSourceGit,
				Git: &RuntimeGitSpec{
					Repo: "https://github.com/myorg/cri-o.git",
					Ref:  "main",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - missing config",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourceGit,
			},
			wantErr: true,
			errMsg:  "git source requires",
		},
		{
			name: "Git source - missing ref",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourceGit,
				Git:     &RuntimeGitSpec{},
			},
			wantErr: true,
			errMsg:  "ref",
		},
		{
			name: "Latest source - default",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourceLatest,
			},
			wantErr: false,
		},
		{
			name: "Latest source - with config",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  RuntimeSourceLatest,
				Latest: &RuntimeLatestSpec{
					Track: "release/1.7",
				},
			},
			wantErr: false,
		},
		{
			name: "Unknown source",
			cr: ContainerRuntime{
				Install: true,
				Name:    ContainerRuntimeContainerd,
				Source:  "unknown",
			},
			wantErr: true,
			errMsg:  "unknown container runtime source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cr.Validate()
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

func TestNVIDIAContainerToolkit_Validate(t *testing.T) {
	tests := []struct {
		name    string
		nct     NVIDIAContainerToolkit
		wantErr bool
		errMsg  string
	}{
		{
			name: "Install disabled - always valid",
			nct: NVIDIAContainerToolkit{
				Install: false,
			},
			wantErr: false,
		},
		{
			name: "Package source - default (no config)",
			nct: NVIDIAContainerToolkit{
				Install: true,
			},
			wantErr: false,
		},
		{
			name: "Package source - explicit",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourcePackage,
				Package: &CTKPackageSpec{
					Channel: "stable",
					Version: "1.17.3-1",
				},
			},
			wantErr: false,
		},
		{
			name: "Package source - experimental channel",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourcePackage,
				Package: &CTKPackageSpec{
					Channel: "experimental",
				},
			},
			wantErr: false,
		},
		{
			name: "Package source - invalid channel",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourcePackage,
				Package: &CTKPackageSpec{
					Channel: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid CTK package channel",
		},
		{
			name: "Git source - valid",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceGit,
				Git: &CTKGitSpec{
					Ref: "v1.17.3",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - with custom repo",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceGit,
				Git: &CTKGitSpec{
					Repo: "https://github.com/myorg/toolkit.git",
					Ref:  "main",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - missing config",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceGit,
			},
			wantErr: true,
			errMsg:  "git source requires",
		},
		{
			name: "Git source - missing ref",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceGit,
				Git:     &CTKGitSpec{},
			},
			wantErr: true,
			errMsg:  "ref",
		},
		{
			name: "Latest source - default",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceLatest,
			},
			wantErr: false,
		},
		{
			name: "Latest source - with config",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  CTKSourceLatest,
				Latest: &CTKLatestSpec{
					Track: "release-1.17",
				},
			},
			wantErr: false,
		},
		{
			name: "Unknown source",
			nct: NVIDIAContainerToolkit{
				Install: true,
				Source:  "unknown",
			},
			wantErr: true,
			errMsg:  "unknown CTK source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.nct.Validate()
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

func TestKubernetes_Validate(t *testing.T) {
	tests := []struct {
		name    string
		k8s     Kubernetes
		wantErr bool
		errMsg  string
	}{
		{
			name: "Install disabled - always valid",
			k8s: Kubernetes{
				Install: false,
			},
			wantErr: false,
		},
		{
			name: "Release source - default (no config)",
			k8s: Kubernetes{
				Install: true,
			},
			wantErr: false,
		},
		{
			name: "Release source - explicit with version",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceRelease,
				Release: &K8sReleaseSpec{
					Version: "v1.31.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Release source - legacy KubernetesVersion field",
			k8s: Kubernetes{
				Install:           true,
				KubernetesVersion: "v1.31.0",
			},
			wantErr: false,
		},
		{
			name: "Git source - valid with kubeadm",
			k8s: Kubernetes{
				Install:             true,
				Source:              K8sSourceGit,
				KubernetesInstaller: "kubeadm",
				Git: &K8sGitSpec{
					Ref: "v1.32.0-alpha.1",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - valid with kind",
			k8s: Kubernetes{
				Install:             true,
				Source:              K8sSourceGit,
				KubernetesInstaller: "kind",
				Git: &K8sGitSpec{
					Ref: "refs/pull/123456/head",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - with custom repo",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceGit,
				Git: &K8sGitSpec{
					Repo: "https://github.com/myorg/kubernetes.git",
					Ref:  "feature/my-feature",
				},
			},
			wantErr: false,
		},
		{
			name: "Git source - missing config",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceGit,
			},
			wantErr: true,
			errMsg:  "git source requires",
		},
		{
			name: "Git source - missing ref",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceGit,
				Git:     &K8sGitSpec{},
			},
			wantErr: true,
			errMsg:  "ref",
		},
		{
			name: "Git source - not supported with microk8s",
			k8s: Kubernetes{
				Install:             true,
				Source:              K8sSourceGit,
				KubernetesInstaller: "microk8s",
				Git: &K8sGitSpec{
					Ref: "v1.32.0-alpha.1",
				},
			},
			wantErr: true,
			errMsg:  "not supported with microk8s",
		},
		{
			name: "Latest source - default",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceLatest,
			},
			wantErr: false,
		},
		{
			name: "Latest source - with config",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceLatest,
				Latest: &K8sLatestSpec{
					Track: "release-1.31",
				},
			},
			wantErr: false,
		},
		{
			name: "Latest source - track master with custom repo",
			k8s: Kubernetes{
				Install: true,
				Source:  K8sSourceLatest,
				Latest: &K8sLatestSpec{
					Track: "master",
					Repo:  "https://github.com/myorg/kubernetes.git",
				},
			},
			wantErr: false,
		},
		{
			name: "Latest source - not supported with microk8s",
			k8s: Kubernetes{
				Install:             true,
				Source:              K8sSourceLatest,
				KubernetesInstaller: "microk8s",
			},
			wantErr: true,
			errMsg:  "not supported with microk8s",
		},
		{
			name: "Unknown source",
			k8s: Kubernetes{
				Install: true,
				Source:  "unknown",
			},
			wantErr: true,
			errMsg:  "unknown Kubernetes source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.k8s.Validate()
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
