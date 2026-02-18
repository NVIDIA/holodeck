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

	assert.Contains(t, out, "COMPONENT=\"docker\"")
	assert.Contains(t, out, "SOURCE=\"package\"")
	assert.Contains(t, out, "DESIRED_VERSION=\"20.10.7\"")
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "systemctl enable docker")
	assert.Contains(t, out, "CRI_DOCKERD_VERSION=\"0.3.17\"")
	assert.Contains(t, out, "sudo tar xzv -C /usr/local/bin --strip-components=1")
	assert.Contains(t, out, "systemctl enable cri-docker.service")
	assert.Contains(t, out, "systemctl enable cri-docker.socket")
	assert.Contains(t, out, "systemctl start cri-docker.service")
	assert.Contains(t, out, "holodeck_verify_docker")
	assert.Contains(t, out, "holodeck_mark_installed")
}

func TestDocker_Execute_PackageSource_OSFamilyBranching(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "24.0.0",
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = d.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	// Must use OS-family branching pattern (not hardcoded apt-get)
	assert.Contains(t, out, "${HOLODECK_OS_FAMILY}",
		"template must branch on HOLODECK_OS_FAMILY")

	// Must use pkg_update abstraction instead of raw apt-get update
	assert.Contains(t, out, "pkg_update",
		"template must use pkg_update instead of raw apt-get update")

	// Must NOT have bare apt-get update (should be inside debian case or use pkg_update)
	assert.NotContains(t, out, "holodeck_retry 3 \"$COMPONENT\" sudo apt-get update",
		"template must not use raw apt-get update outside OS-family branching")

	// Debian path: GPG key and apt sources
	assert.Contains(t, out, "debian)",
		"template must have debian case branch")
	assert.Contains(t, out, "/etc/apt/keyrings/docker.gpg",
		"debian branch must set up Docker GPG key")
	assert.Contains(t, out, "download.docker.com/linux/ubuntu",
		"debian branch must use Docker Ubuntu repo")

	// RPM path: Docker repo setup
	assert.Contains(t, out, "amazon|rhel)",
		"template must have amazon|rhel case branch")
	assert.Contains(t, out, "/etc/yum.repos.d/docker-ce.repo",
		"RPM branch must set up docker-ce.repo")

	// Amazon Linux: Fedora repo mapping
	assert.Contains(t, out, "HOLODECK_AMZN_FEDORA_VERSION",
		"RPM branch must use HOLODECK_AMZN_FEDORA_VERSION for Amazon Linux")
	assert.Contains(t, out, "download.docker.com/linux/fedora/docker-ce.repo",
		"Amazon Linux must use Fedora Docker repo")

	// Rocky/RHEL/CentOS: CentOS repo
	assert.Contains(t, out, "download.docker.com/linux/centos/docker-ce.repo",
		"RHEL-family must use CentOS Docker repo")

	// Unsupported OS family error
	assert.Contains(t, out, "Unsupported OS family",
		"template must error on unsupported OS family")
}

func TestDocker_Execute_PackageSource_RPMVersionSyntax(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "24.0.0",
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = d.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	// Debian uses = for version pinning (docker-ce=$VERSION)
	assert.Contains(t, out, "docker-ce=$DESIRED_VERSION",
		"debian branch must use = for version pinning")

	// RPM uses - for version pinning (docker-ce-$VERSION)
	assert.Contains(t, out, "docker-ce-$DESIRED_VERSION",
		"RPM branch must use - for version pinning")
}

func TestDocker_Execute_PackageSource_SourcesOsRelease(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "latest",
			},
		},
	}
	d, err := NewDocker(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = d.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	// Must source /etc/os-release for ${ID} variable used in RPM case
	assert.Contains(t, out, ". /etc/os-release",
		"template must source /etc/os-release for OS detection variables")
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

	assert.Contains(t, out, "SOURCE=\"git\"")
	assert.Contains(t, out, "GIT_REF=\"v24.0.0\"")
	assert.Contains(t, out, "GIT_COMMIT=\"abc12345\"")
	assert.Contains(t, out, "hack/make.sh binary")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_docker")
}
