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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewDocker_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	d, err := NewDocker(env)
	require.NoError(t, err)
	assert.Equal(t, "package", d.Source)
	assert.Equal(t, "latest", d.Version)
}

func TestNewDocker_CustomVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "20.10.7",
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	assert.Equal(t, "20.10.7", d.Version)
}

func TestNewDocker_PackageSpec(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeDocker,
				Source:  v1alpha1.RuntimeSourcePackage,
				Package: &v1alpha1.RuntimePackageSpec{
					Version: "24.0.0",
				},
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	assert.Equal(t, "package", d.Source)
	assert.Equal(t, "24.0.0", d.Version)
}

func TestNewDocker_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeDocker,
				Source:  v1alpha1.RuntimeSourceGit,
				Git: &v1alpha1.RuntimeGitSpec{
					Ref: "v24.0.0",
				},
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	assert.Equal(t, "git", d.Source)
	assert.Equal(t, "v24.0.0", d.GitRef)
	assert.Equal(t, "https://github.com/moby/moby.git", d.GitRepo)
}

func TestNewDocker_GitSourceMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Source:  v1alpha1.RuntimeSourceGit,
			},
		},
	}
	_, err := NewDocker(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git source requires")
}

func TestDocker_Execute_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "20.10.7",
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = d.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `COMPONENT="docker"`)
	assert.Contains(t, out, `SOURCE="package"`)
	assert.Contains(t, out, `DESIRED_VERSION="20.10.7"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "docker-ce=$DESIRED_VERSION")
	assert.Contains(t, out, "systemctl enable docker")
	assert.Contains(t, out, `CRI_DOCKERD_VERSION="0.3.17"`)
	assert.Contains(t, out, "sudo tar xzv -C /usr/local/bin --strip-components=1")
	assert.Contains(t, out, "systemctl enable cri-docker.service")
	assert.Contains(t, out, "systemctl enable cri-docker.socket")
	assert.Contains(t, out, "systemctl start cri-docker.service")
	assert.Contains(t, out, "holodeck_verify_docker")
	assert.Contains(t, out, "holodeck_mark_installed")
}

func TestDocker_Execute_GitSource(t *testing.T) {
	d := &Docker{
		Source:    "git",
		GitRepo:   "https://github.com/moby/moby.git",
		GitRef:    "v24.0.0",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := d.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `SOURCE="git"`)
	assert.Contains(t, out, `GIT_REF="v24.0.0"`)
	assert.Contains(t, out, `GIT_COMMIT="abc12345"`)
	assert.Contains(t, out, "hack/make.sh binary")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_docker")
}
