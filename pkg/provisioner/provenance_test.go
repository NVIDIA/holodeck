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
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestBuildComponentsStatus_Empty(t *testing.T) {
	env := v1alpha1.Environment{}
	cs := BuildComponentsStatus(env)
	assert.Nil(t, cs, "no components installed should return nil")
}

func TestBuildComponentsStatus_DriverPackage(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Branch:  "560",
				Version: "560.35.03",
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Driver)
	assert.Equal(t, "package", cs.Driver.Source)
	assert.Equal(t, "560", cs.Driver.Branch)
	assert.Equal(t, "560.35.03", cs.Driver.Version)
	assert.Nil(t, cs.Runtime)
	assert.Nil(t, cs.Toolkit)
	assert.Nil(t, cs.Kubernetes)
}

func TestBuildComponentsStatus_RuntimePackage(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeContainerd,
				Version: "1.7.20",
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Runtime)
	assert.Equal(t, "package", cs.Runtime.Source)
	assert.Equal(t, "1.7.20", cs.Runtime.Version)
}

func TestBuildComponentsStatus_ToolkitPackage(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Version: "1.17.3-1",
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Toolkit)
	assert.Equal(t, "package", cs.Toolkit.Source)
	assert.Equal(t, "1.17.3-1", cs.Toolkit.Version)
}

func TestBuildComponentsStatus_ToolkitGit(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceGit,
				Git: &v1alpha1.CTKGitSpec{
					Repo: "https://github.com/NVIDIA/nvidia-container-toolkit.git",
					Ref:  "v1.17.3",
				},
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Toolkit)
	assert.Equal(t, "git", cs.Toolkit.Source)
	assert.Equal(t, "https://github.com/NVIDIA/nvidia-container-toolkit.git", cs.Toolkit.Repo)
	assert.Equal(t, "v1.17.3", cs.Toolkit.Ref)
}

func TestBuildComponentsStatus_ToolkitLatest(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceLatest,
				Latest: &v1alpha1.CTKLatestSpec{
					Track: "main",
					Repo:  "https://github.com/NVIDIA/nvidia-container-toolkit.git",
				},
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Toolkit)
	assert.Equal(t, "latest", cs.Toolkit.Source)
	assert.Equal(t, "main", cs.Toolkit.Branch)
	assert.Equal(t, "https://github.com/NVIDIA/nvidia-container-toolkit.git", cs.Toolkit.Repo)
}

func TestBuildComponentsStatus_KubernetesRelease(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install: true,
				Release: &v1alpha1.K8sReleaseSpec{
					Version: "v1.31.1",
				},
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Kubernetes)
	assert.Equal(t, "release", cs.Kubernetes.Source)
	assert.Equal(t, "v1.31.1", cs.Kubernetes.Version)
}

func TestBuildComponentsStatus_KubernetesGit(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install: true,
				Source:  v1alpha1.K8sSourceGit,
				Git: &v1alpha1.K8sGitSpec{
					Ref: "refs/heads/master",
				},
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Kubernetes)
	assert.Equal(t, "git", cs.Kubernetes.Source)
	assert.Equal(t, "refs/heads/master", cs.Kubernetes.Ref)
}

func TestBuildComponentsStatus_KubernetesLegacyVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install:           true,
				KubernetesVersion: "v1.30.0",
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	require.NotNil(t, cs.Kubernetes)
	assert.Equal(t, "release", cs.Kubernetes.Source)
	assert.Equal(t, "v1.30.0", cs.Kubernetes.Version)
}

func TestBuildComponentsStatus_AllComponents(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Branch:  "575",
			},
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeContainerd,
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
			},
			Kubernetes: v1alpha1.Kubernetes{
				Install: true,
				Release: &v1alpha1.K8sReleaseSpec{
					Version: "v1.31.1",
				},
			},
		},
	}
	cs := BuildComponentsStatus(env)
	require.NotNil(t, cs)
	assert.NotNil(t, cs.Driver)
	assert.NotNil(t, cs.Runtime)
	assert.NotNil(t, cs.Toolkit)
	assert.NotNil(t, cs.Kubernetes)
}
