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

package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewContainerToolkit_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "containerd", ctk.ContainerRuntime)
	assert.False(t, ctk.EnableCDI)
	assert.Equal(t, "package", ctk.Source)
	assert.Equal(t, "stable", ctk.Channel)
}

func TestNewContainerToolkit_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "docker",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install:   true,
				Source:    v1alpha1.CTKSourcePackage,
				EnableCDI: true,
				Package: &v1alpha1.CTKPackageSpec{
					Channel: "experimental",
					Version: "1.17.3-1",
				},
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "docker", ctk.ContainerRuntime)
	assert.True(t, ctk.EnableCDI)
	assert.Equal(t, "package", ctk.Source)
	assert.Equal(t, "experimental", ctk.Channel)
	assert.Equal(t, "1.17.3-1", ctk.Version)
}

func TestNewContainerToolkit_LegacyVersion(t *testing.T) {
	// Test backward compatibility with legacy Version field
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Version: "1.16.0-1", // Legacy field
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "package", ctk.Source)
	assert.Equal(t, "1.16.0-1", ctk.Version)
	assert.Equal(t, "stable", ctk.Channel)
}

func TestNewContainerToolkit_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install:   true,
				Source:    v1alpha1.CTKSourceGit,
				EnableCDI: true,
				Git: &v1alpha1.CTKGitSpec{
					Ref: "v1.17.3",
				},
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "git", ctk.Source)
	assert.Equal(t, "v1.17.3", ctk.GitRef)
	assert.Equal(t,
		"https://github.com/NVIDIA/nvidia-container-toolkit.git",
		ctk.GitRepo)
}

func TestNewContainerToolkit_GitSourceCustomRepo(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceGit,
				Git: &v1alpha1.CTKGitSpec{
					Repo: "https://github.com/myorg/nvidia-container-toolkit.git",
					Ref:  "abc123",
				},
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "git", ctk.Source)
	assert.Equal(t, "abc123", ctk.GitRef)
	assert.Equal(t,
		"https://github.com/myorg/nvidia-container-toolkit.git",
		ctk.GitRepo)
}

func TestNewContainerToolkit_GitSourceMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceGit,
				// Missing Git config
			},
		},
	}
	_, err := NewContainerToolkit(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git source requires")
}

func TestNewContainerToolkit_LatestSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install:   true,
				Source:    v1alpha1.CTKSourceLatest,
				EnableCDI: true,
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "latest", ctk.Source)
	assert.Equal(t, "main", ctk.TrackBranch)
	assert.Equal(t,
		"https://github.com/NVIDIA/nvidia-container-toolkit.git",
		ctk.GitRepo)
}

func TestNewContainerToolkit_LatestSourceCustomBranch(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceLatest,
				Latest: &v1alpha1.CTKLatestSpec{
					Track: "release-1.17",
				},
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	assert.Equal(t, "latest", ctk.Source)
	assert.Equal(t, "release-1.17", ctk.TrackBranch)
}

func TestContainerToolkit_Execute_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install:   true,
				EnableCDI: true,
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ctk.Execute(&buf, env)
	require.NoError(t, err)

	out := buf.String()

	// Test package source specifics
	assert.Contains(t, out, `SOURCE="package"`)
	assert.Contains(t, out, `CHANNEL="stable"`)
	assert.Contains(t, out, `COMPONENT="nvidia-container-toolkit"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, `--runtime="${CONTAINER_RUNTIME}"`)
	assert.Contains(t, out, `--enable-cdi="${ENABLE_CDI}"`)
	assert.Contains(t, out, "holodeck_verify_toolkit")
	assert.Contains(t, out, "holodeck_mark_installed")
	assert.Contains(t, out, "PROVENANCE.json")
}

func TestContainerToolkit_Execute_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceGit,
				Git: &v1alpha1.CTKGitSpec{
					Ref: "v1.17.3",
				},
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)
	ctk.SetResolvedCommit("abc12345")

	var buf bytes.Buffer
	err = ctk.Execute(&buf, env)
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `SOURCE="git"`)
	assert.Contains(t, out, `GIT_REF="v1.17.3"`)
	assert.Contains(t, out, `GIT_COMMIT="abc12345"`)
	assert.Contains(t, out, "ghcr.io/nvidia/container-toolkit")
	assert.Contains(t, out, "PROVENANCE.json")
}

func TestContainerToolkit_Execute_LatestSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
				Source:  v1alpha1.CTKSourceLatest,
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ctk.Execute(&buf, env)
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `SOURCE="latest"`)
	assert.Contains(t, out, `TRACK_BRANCH="main"`)
	assert.Contains(t, out, "git ls-remote")
	assert.Contains(t, out, "PROVENANCE.json")
}

func TestContainerToolkit_SetResolvedCommit(t *testing.T) {
	ctk := &ContainerToolkit{}
	ctk.SetResolvedCommit("abc12345")
	assert.Equal(t, "abc12345", ctk.GitCommit)
}

func TestContainerToolkit_Execute_CNIVerification(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
			},
		},
	}
	ctk, err := NewContainerToolkit(env)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ctk.Execute(&buf, env)
	require.NoError(t, err)

	out := buf.String()

	// Verify CNI path preservation is in the template
	if !strings.Contains(out, `bin_dir = "/opt/cni/bin"`) {
		t.Error("template output missing correct CNI bin_dir check")
	}
}
