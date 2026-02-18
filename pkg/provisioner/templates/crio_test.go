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

func TestNewCriO_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	c, err := NewCriO(env)
	require.NoError(t, err)
	assert.Equal(t, "package", c.Source)
	assert.Equal(t, "", c.Version)
}

func TestNewCriO_CustomVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.25",
			},
		},
	}
	c, err := NewCriO(env)
	require.NoError(t, err)
	assert.Equal(t, "1.25", c.Version)
}

func TestNewCriO_PackageSpec(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeCrio,
				Source:  v1alpha1.RuntimeSourcePackage,
				Package: &v1alpha1.RuntimePackageSpec{
					Version: "1.30",
				},
			},
		},
	}
	c, err := NewCriO(env)
	require.NoError(t, err)
	assert.Equal(t, "package", c.Source)
	assert.Equal(t, "1.30", c.Version)
}

func TestNewCriO_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeCrio,
				Source:  v1alpha1.RuntimeSourceGit,
				Git: &v1alpha1.RuntimeGitSpec{
					Ref: "v1.30.0",
				},
			},
		},
	}
	c, err := NewCriO(env)
	require.NoError(t, err)
	assert.Equal(t, "git", c.Source)
	assert.Equal(t, "v1.30.0", c.GitRef)
	assert.Equal(t, "https://github.com/cri-o/cri-o.git", c.GitRepo)
}

func TestNewCriO_GitSourceMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Source:  v1alpha1.RuntimeSourceGit,
			},
		},
	}
	_, err := NewCriO(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git source requires")
}

func TestCriO_Execute_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.25",
			},
		},
	}
	c, err := NewCriO(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = c.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `COMPONENT="crio"`)
	assert.Contains(t, out, `SOURCE="package"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "pkg_install cri-o")
	assert.Contains(t, out, "systemctl start crio.service")
	assert.Contains(t, out, "holodeck_verify_crio")
	assert.Contains(t, out, "holodeck_mark_installed")
}

func TestCriO_Execute_GitSource(t *testing.T) {
	c := &CriO{
		Source:    "git",
		GitRepo:   "https://github.com/cri-o/cri-o.git",
		GitRef:    "v1.30.0",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `SOURCE="git"`)
	assert.Contains(t, out, `GIT_REF="v1.30.0"`)
	assert.Contains(t, out, `GIT_COMMIT="abc12345"`)
	assert.Contains(t, out, "make")
	assert.Contains(t, out, "make install")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_crio")
}

// === RPM SUPPORT TESTS ===

func TestCriO_Execute_PackageTemplate_OSFamilyBranching(t *testing.T) {
	c := &CriO{
		Source:  "package",
		Version: "1.30",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// Must contain OS-family branching
	assert.Contains(t, out, `case "${HOLODECK_OS_FAMILY}" in`,
		"Package template must branch on HOLODECK_OS_FAMILY")
	assert.Contains(t, out, "debian)",
		"Package template must handle debian OS family")
	assert.Contains(t, out, "amazon|rhel)",
		"Package template must handle amazon and rhel OS families")

	// Must contain unsupported OS family error
	assert.Contains(t, out, "Unsupported OS family",
		"Package template must error on unsupported OS families")
}

func TestCriO_Execute_PackageTemplate_DebianRepoSetup(t *testing.T) {
	c := &CriO{
		Source:  "package",
		Version: "1.30",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// Debian still uses GPG keyring and apt sources
	assert.Contains(t, out, "cri-o-apt-keyring.gpg",
		"Package template must set up GPG keyring for Debian")
	assert.Contains(t, out, "sources.list.d/cri-o.list",
		"Package template must configure apt source for Debian")
}

func TestCriO_Execute_PackageTemplate_RPMRepoSetup(t *testing.T) {
	c := &CriO{
		Source:  "package",
		Version: "1.30",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// RPM uses yum.repos.d
	assert.Contains(t, out, "yum.repos.d",
		"Package template must set up yum repo for RPM")
	assert.Contains(t, out, "/rpm/",
		"Package template must reference RPM repo URL")
}

func TestCriO_Execute_GitTemplate_OSFamilyBranching(t *testing.T) {
	c := &CriO{
		Source:    "git",
		GitRepo:   "https://github.com/cri-o/cri-o.git",
		GitRef:    "v1.30.0",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// Git template must have OS-family branching for build deps
	assert.Contains(t, out, `case "${HOLODECK_OS_FAMILY}" in`,
		"Git template must branch on HOLODECK_OS_FAMILY")
	assert.Contains(t, out, "debian)",
		"Git template must handle debian OS family")
	assert.Contains(t, out, "amazon|rhel)",
		"Git template must handle amazon and rhel OS families")
}

func TestCriO_Execute_GitTemplate_RPMBuildDeps(t *testing.T) {
	c := &CriO{
		Source:    "git",
		GitRepo:   "https://github.com/cri-o/cri-o.git",
		GitRef:    "v1.30.0",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// RPM build deps
	assert.Contains(t, out, "libseccomp-devel",
		"Git template must install libseccomp-devel for RPM")
	assert.Contains(t, out, "gpgme-devel",
		"Git template must install gpgme-devel for RPM")
	assert.Contains(t, out, "glib2-devel",
		"Git template must install glib2-devel for RPM")

	// Debian build deps still present
	assert.Contains(t, out, "libseccomp-dev",
		"Git template must still install libseccomp-dev for Debian")
	assert.Contains(t, out, "libgpgme-dev",
		"Git template must still install libgpgme-dev for Debian")
}
