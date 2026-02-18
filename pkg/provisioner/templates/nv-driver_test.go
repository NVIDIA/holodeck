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

// === RPM SUPPORT TESTS ===

func TestNvDriver_Execute_PackageTemplate_OSFamilyBranching(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: defaultNVBranch,
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
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

	// Must use pkg_update abstraction instead of raw apt-get update
	assert.Contains(t, out, "pkg_update",
		"Package template must use pkg_update abstraction")
	assert.NotContains(t, out, "sudo apt-get update",
		"Package template must not use raw apt-get update")
}

func TestNvDriver_Execute_PackageTemplate_RPMKernelHeaders(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: defaultNVBranch,
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// RPM uses kernel-devel and kernel-headers instead of linux-headers
	assert.Contains(t, out, "kernel-devel",
		"Package template must install kernel-devel for RPM-based systems")
	assert.Contains(t, out, "kernel-headers",
		"Package template must install kernel-headers for RPM-based systems")

	// Debian still uses linux-headers
	assert.Contains(t, out, "linux-headers-${KERNEL_VERSION}",
		"Package template must still install linux-headers for Debian")
}

func TestNvDriver_Execute_PackageTemplate_RPMBuildDeps(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: defaultNVBranch,
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// RPM build deps
	assert.Contains(t, out, "elfutils-libelf-devel",
		"Package template must install elfutils-libelf-devel for RPM")
	assert.Contains(t, out, "mesa-libEGL-devel",
		"Package template must install mesa-libEGL-devel for RPM")
	assert.Contains(t, out, "gcc-c++",
		"Package template must install gcc-c++ for RPM")

	// Debian build deps still present
	assert.Contains(t, out, "build-essential",
		"Package template must still install build-essential for Debian")
	assert.Contains(t, out, "libelf-dev",
		"Package template must still install libelf-dev for Debian")
}

func TestNvDriver_Execute_PackageTemplate_RPMCUDARepo(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: defaultNVBranch,
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// RPM CUDA repo uses .repo file
	assert.Contains(t, out, ".repo",
		"Package template must set up .repo file for RPM")

	// Debian CUDA repo still uses .deb keyring
	assert.Contains(t, out, "cuda-keyring",
		"Package template must still use cuda-keyring for Debian")
	assert.Contains(t, out, "dpkg -i",
		"Package template must still use dpkg for Debian")
}

func TestNvDriver_Execute_PackageTemplate_SourcesOSRelease(t *testing.T) {
	nvd := &NvDriver{
		Source: "package",
		Branch: defaultNVBranch,
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// Must source os-release for distro detection
	assert.Contains(t, out, `. /etc/os-release`,
		"Package template must source os-release for distro detection")
}

func TestNvDriver_Execute_RunfileTemplate_OSFamilyBranching(t *testing.T) {
	nvd := &NvDriver{
		Source:     "runfile",
		RunfileURL: "https://download.nvidia.com/driver.run",
	}

	var buf bytes.Buffer
	err := nvd.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)

	out := buf.String()

	// Runfile template must have OS-family branching for dependencies
	assert.Contains(t, out, `case "${HOLODECK_OS_FAMILY}" in`,
		"Runfile template must branch on HOLODECK_OS_FAMILY")
	assert.Contains(t, out, "kernel-devel",
		"Runfile template must install kernel-devel for RPM")
	assert.Contains(t, out, "elfutils-libelf-devel",
		"Runfile template must install elfutils-libelf-devel for RPM")

	// Must use pkg_update abstraction
	assert.Contains(t, out, "pkg_update",
		"Runfile template must use pkg_update abstraction")
	assert.NotContains(t, out, "sudo apt-get update",
		"Runfile template must not use raw apt-get update")
}

func TestNvDriver_Execute_GitTemplate_OSFamilyBranching(t *testing.T) {
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

	// Git template must have OS-family branching for dependencies
	assert.Contains(t, out, `case "${HOLODECK_OS_FAMILY}" in`,
		"Git template must branch on HOLODECK_OS_FAMILY")
	assert.Contains(t, out, "kernel-devel",
		"Git template must install kernel-devel for RPM")
	assert.Contains(t, out, "elfutils-libelf-devel",
		"Git template must install elfutils-libelf-devel for RPM")
	assert.Contains(t, out, "gcc-c++",
		"Git template must install gcc-c++ for RPM")

	// Must use pkg_update abstraction
	assert.Contains(t, out, "pkg_update",
		"Git template must use pkg_update abstraction")
	assert.NotContains(t, out, "sudo apt-get update",
		"Git template must not use raw apt-get update")
}

func TestNvDriver_Execute_NoRedundantRedirects(t *testing.T) {
	sources := []struct {
		name string
		nvd  NvDriver
	}{
		{"package", NvDriver{Source: "package", Branch: defaultNVBranch}},
		{"runfile", NvDriver{Source: "runfile", RunfileURL: "https://download.nvidia.com/driver.run"}},
		{"git", NvDriver{Source: "git", GitRepo: "https://github.com/NVIDIA/open-gpu-kernel-modules.git", GitRef: "560.35.03", GitCommit: "abc12345"}},
	}
	for _, tc := range sources {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tc.nvd.Execute(&buf, v1alpha1.Environment{})
			require.NoError(t, err)
			assert.NotContains(t, buf.String(), "&>/dev/null 2>&1",
				"Template must not use redundant redirect &>/dev/null 2>&1; use &>/dev/null alone")
		})
	}
}
