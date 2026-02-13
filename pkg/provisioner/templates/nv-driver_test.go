/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

func TestNewNvDriver_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "package", nvd.Source)
	assert.Equal(t, defaultNVBranch, nvd.Branch)
	assert.Equal(t, "", nvd.Version)
}

func TestNewNvDriver_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourcePackage,
				Package: &v1alpha1.DriverPackageSpec{
					Branch:  "560",
					Version: "560.35.03",
				},
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "package", nvd.Source)
	assert.Equal(t, "560", nvd.Branch)
	assert.Equal(t, "560.35.03", nvd.Version)
}

func TestNewNvDriver_PackageLegacyFields(t *testing.T) {
	// Test backward compatibility with legacy Branch/Version fields
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Branch:  "550",
				Version: "550.90.07",
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "package", nvd.Source)
	assert.Equal(t, "550", nvd.Branch)
	assert.Equal(t, "550.90.07", nvd.Version)
}

func TestNewNvDriver_PackageDefaultBranch(t *testing.T) {
	// When no package config and no legacy fields, use default branch
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourcePackage,
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, defaultNVBranch, nvd.Branch)
	assert.Equal(t, "", nvd.Version)
}

func TestNewNvDriver_PackageVersionOnly(t *testing.T) {
	// Version provided but no branch - no default branch applied
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Version: "535.129.03", // legacy field
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "", nvd.Branch)
	assert.Equal(t, "535.129.03", nvd.Version)
}

func TestNewNvDriver_RunfileSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourceRunfile,
				Runfile: &v1alpha1.DriverRunfileSpec{
					URL:      "https://download.nvidia.com/driver.run",
					Checksum: "sha256:abc123",
				},
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "runfile", nvd.Source)
	assert.Equal(t, "https://download.nvidia.com/driver.run", nvd.RunfileURL)
	assert.Equal(t, "sha256:abc123", nvd.RunfileChecksum)
}

func TestNewNvDriver_RunfileMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourceRunfile,
				// Missing Runfile config
			},
		},
	}
	_, err := NewNvDriver(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runfile source requires")
}

func TestNewNvDriver_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourceGit,
				Git: &v1alpha1.DriverGitSpec{
					Ref: "560.35.03",
				},
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "git", nvd.Source)
	assert.Equal(t, "560.35.03", nvd.GitRef)
	assert.Equal(t,
		"https://github.com/NVIDIA/open-gpu-kernel-modules.git",
		nvd.GitRepo)
}

func TestNewNvDriver_GitSourceCustomRepo(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourceGit,
				Git: &v1alpha1.DriverGitSpec{
					Repo: "https://github.com/myorg/open-gpu-kernel-modules.git",
					Ref:  "main",
				},
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)
	assert.Equal(t, "git", nvd.Source)
	assert.Equal(t, "main", nvd.GitRef)
	assert.Equal(t,
		"https://github.com/myorg/open-gpu-kernel-modules.git",
		nvd.GitRepo)
}

func TestNewNvDriver_GitSourceMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
				Source:  v1alpha1.DriverSourceGit,
				// Missing Git config
			},
		},
	}
	_, err := NewNvDriver(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git source requires")
}

func TestNvDriver_SetResolvedCommit(t *testing.T) {
	nvd := &NvDriver{}
	nvd.SetResolvedCommit("abc12345")
	assert.Equal(t, "abc12345", nvd.GitCommit)
}

func TestNvDriver_Execute_PackageSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
			},
		},
	}
	nvd, err := NewNvDriver(env)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = nvd.Execute(&buf, env)
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `SOURCE="package"`)
	assert.Contains(t, out, `COMPONENT="nvidia-driver"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "holodeck_verify_driver")
	assert.Contains(t, out, "holodeck_mark_installed")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "cuda-drivers")
}

func TestNvDriver_Execute_PackageWithVersion(t *testing.T) {
	nvd := &NvDriver{
		Source:  "package",
		Version: "560.35.03",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `DESIRED_VERSION="560.35.03"`)
	assert.Contains(t, out, `DRIVER_PACKAGE="${DRIVER_PACKAGE}=${DESIRED_VERSION}"`)
}

func TestNvDriver_Execute_PackageWithBranch(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: "550",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `DESIRED_BRANCH="550"`)
	assert.Contains(t, out, `DRIVER_PACKAGE="${DRIVER_PACKAGE}-${DESIRED_BRANCH}"`)
}

func TestNvDriver_Execute_RunfileSource(t *testing.T) {
	nvd := &NvDriver{
		Source:          "runfile",
		RunfileURL:      "https://download.nvidia.com/driver.run",
		RunfileChecksum: "sha256:abc123",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `SOURCE="runfile"`)
	assert.Contains(t, out, `RUNFILE_URL="https://download.nvidia.com/driver.run"`)
	assert.Contains(t, out, `RUNFILE_CHECKSUM="sha256:abc123"`)
	assert.Contains(t, out, "sha256sum")
	assert.Contains(t, out, "--silent --dkms")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_driver")
}

func TestNvDriver_Execute_GitSource(t *testing.T) {
	nvd := &NvDriver{
		Source:    "git",
		GitRepo:   "https://github.com/NVIDIA/open-gpu-kernel-modules.git",
		GitRef:    "560.35.03",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, `SOURCE="git"`)
	assert.Contains(t, out, `GIT_REF="560.35.03"`)
	assert.Contains(t, out, `GIT_COMMIT="abc12345"`)
	assert.Contains(t, out, "make modules")
	assert.Contains(t, out, "make modules_install")
	assert.Contains(t, out, "depmod")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_driver")
}

func TestNvDriver_Execute_UnknownSource(t *testing.T) {
	nvd := &NvDriver{
		Source: "invalid",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver source")
}

func TestNVDriverTemplate_CUDARepoArch(t *testing.T) {
	driver := &NvDriver{
		Branch: defaultNVBranch,
	}

	var output bytes.Buffer
	err := driver.Execute(&output, v1alpha1.Environment{})
	require.NoError(t, err)

	outStr := output.String()

	// Must NOT contain hardcoded x86_64 in the CUDA repo URL
	require.NotContains(t, outStr, "cuda/repos/$distribution/x86_64/",
		"Template must not hardcode x86_64 in the CUDA repository URL")

	// Must contain runtime architecture detection
	require.Contains(t, outStr, `CUDA_ARCH="$(uname -m)"`,
		"Template must detect architecture at runtime via uname -m")

	// Must contain aarch64 -> sbsa mapping
	require.Contains(t, outStr, `if [[ "$CUDA_ARCH" == "aarch64" ]]; then`,
		"Template must check for aarch64 architecture")
	require.Contains(t, outStr, `CUDA_ARCH="sbsa"`,
		"Template must map aarch64 to sbsa for NVIDIA CUDA repos")

	// Must use CUDA_ARCH variable in the wget URL
	require.Contains(t, outStr, "${CUDA_ARCH}/cuda-keyring",
		"Template must use CUDA_ARCH variable in the wget URL")
}
