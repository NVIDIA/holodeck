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
