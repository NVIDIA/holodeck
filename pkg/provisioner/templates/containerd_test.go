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

func TestNewContainerd_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "package", c.Source)
	assert.Equal(t, "1.7.27", c.Version)
	assert.Equal(t, 1, c.MajorVersion)
}

func TestNewContainerd_CustomVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "v1.7.0",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "1.7.0", c.Version)
	assert.Equal(t, 1, c.MajorVersion)
}

func TestNewContainerd_EmptyVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "1.7.27", c.Version)
	assert.Equal(t, 1, c.MajorVersion)
}

func TestNewContainerd_Version2(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "2.0.0",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", c.Version)
	assert.Equal(t, 2, c.MajorVersion)
}

func TestNewContainerd_PackageSpec(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeContainerd,
				Source:  v1alpha1.RuntimeSourcePackage,
				Package: &v1alpha1.RuntimePackageSpec{
					Version: "1.7.20",
				},
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "package", c.Source)
	assert.Equal(t, "1.7.20", c.Version)
}

func TestNewContainerd_GitSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeContainerd,
				Source:  v1alpha1.RuntimeSourceGit,
				Git: &v1alpha1.RuntimeGitSpec{
					Ref: "v1.7.23",
				},
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "git", c.Source)
	assert.Equal(t, "v1.7.23", c.GitRef)
	assert.Equal(t, "https://github.com/containerd/containerd.git", c.GitRepo)
}

func TestNewContainerd_GitSourceMissingConfig(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Source:  v1alpha1.RuntimeSourceGit,
			},
		},
	}
	_, err := NewContainerd(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git source requires")
}

func TestNewContainerd_LatestSource(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    v1alpha1.ContainerRuntimeContainerd,
				Source:  v1alpha1.RuntimeSourceLatest,
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "latest", c.Source)
	assert.Equal(t, "main", c.TrackBranch)
	assert.Equal(t, "https://github.com/containerd/containerd.git", c.GitRepo)
}

func TestNewContainerd_LatestSourceCustomBranch(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Source:  v1alpha1.RuntimeSourceLatest,
				Latest: &v1alpha1.RuntimeLatestSpec{
					Track: "release/1.7",
				},
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	assert.Equal(t, "release/1.7", c.TrackBranch)
}

func TestContainerd_Execute_Version1(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.7.26",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = c.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `COMPONENT="containerd"`)
	assert.Contains(t, out, `SOURCE="package"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "Installing containerd 1.7.26 using package repository")
	assert.Contains(t, out, "1.7.26")
	assert.Contains(t, out, "download.docker.com")
	assert.Contains(t, out, "HOLODECK_AMZN_FEDORA_VERSION")
	assert.NotContains(t, out, `'s/\$releasever/39/g'`)
	assert.Contains(t, out, `SystemdCgroup \= true`)
	assert.Contains(t, out, "containerd config default")
	assert.Contains(t, out, `conf_dir = "/etc/cni/net.d"`)
	assert.Contains(t, out, `bin_dir = "/opt/cni/bin"`)
	assert.Contains(t, out, "holodeck_verify_containerd")
	assert.Contains(t, out, "holodeck_mark_installed")
}

func TestContainerd_Execute_Version2(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "2.0.0",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = c.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `COMPONENT="containerd"`)
	assert.Contains(t, out, "holodeck_progress")
	assert.Contains(t, out, "Installing containerd 2.0.0 from official binaries")
	assert.Contains(t, out, "containerd-2.0.0-linux-${ARCH}.tar.gz")
	assert.Contains(t, out, "https://github.com/containerd/containerd/releases/download/v2.0.0/")
	assert.Contains(t, out, "SystemdCgroup = true")
	assert.Contains(t, out, "containerd config default")
	assert.Contains(t, out, `RUNC_VERSION="1.2.3"`)
	assert.Contains(t, out, `CNI_VERSION="v1.6.2"`)
	assert.Contains(t, out, `conf_dir = "/etc/cni/net.d"`)
	assert.Contains(t, out, `bin_dir = "/opt/cni/bin"`)
	assert.Contains(t, out, "holodeck_verify_containerd")
	assert.Contains(t, out, "holodeck_mark_installed")
}

func TestContainerd_Execute_GitSource(t *testing.T) {
	c := &Containerd{
		Source:    "git",
		GitRepo:   "https://github.com/containerd/containerd.git",
		GitRef:    "v1.7.23",
		GitCommit: "abc12345",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `SOURCE="git"`)
	assert.Contains(t, out, `GIT_REF="v1.7.23"`)
	assert.Contains(t, out, `GIT_COMMIT="abc12345"`)
	assert.Contains(t, out, "make")
	assert.Contains(t, out, "make install")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_containerd")
}

func TestContainerd_Execute_LatestSource(t *testing.T) {
	c := &Containerd{
		Source:      "latest",
		GitRepo:     "https://github.com/containerd/containerd.git",
		TrackBranch: "main",
	}

	var buf bytes.Buffer
	err := c.Execute(&buf, v1alpha1.Environment{})
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, `SOURCE="latest"`)
	assert.Contains(t, out, `TRACK_BRANCH="main"`)
	assert.Contains(t, out, "git ls-remote")
	assert.Contains(t, out, "PROVENANCE.json")
	assert.Contains(t, out, "holodeck_verify_containerd")
}

func TestContainerd_Execute_CommonElements(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"v1 template", "1.7.26"},
		{"v2 template", "2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Version: tt.version,
					},
				},
			}
			c, err := NewContainerd(env)
			require.NoError(t, err)
			var buf bytes.Buffer
			err = c.Execute(&buf, env)
			require.NoError(t, err)
			out := buf.String()

			if tt.version == "2.0.0" {
				assert.True(t, strings.Contains(out, "sudo modprobe overlay"))
				assert.True(t, strings.Contains(out, "sudo modprobe br_netfilter"))
				assert.True(t, strings.Contains(out, "net.bridge.bridge-nf-call-iptables"))
				assert.True(t, strings.Contains(out, "net.ipv4.ip_forward"))
				assert.True(t, strings.Contains(out, "sudo sysctl --system"))
				assert.True(t, strings.Contains(out, `if [[ "$ARCH" == "x86_64" ]]`))
				assert.True(t, strings.Contains(out, `ARCH="amd64"`))
				assert.True(t, strings.Contains(out, `elif [[ "$ARCH" == "aarch64" ]]`))
				assert.True(t, strings.Contains(out, `ARCH="arm64"`))
				assert.True(t, strings.Contains(out, `CNI_VERSION="v1.6.2"`))
				assert.True(t, strings.Contains(out, "/opt/cni/bin"))
			}

			assert.Contains(t, out, "sudo mkdir -p /etc/containerd")
			if tt.version == "1.7.26" {
				assert.Contains(t, out, "sudo systemctl restart containerd")
			} else {
				assert.Contains(t, out, "sudo systemctl enable --now containerd")
			}
			assert.Contains(t, out, "sudo systemctl enable")
		})
	}
}

func TestContainerd_V2Template_UsesPkgUpdate(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "2.0.0",
			},
		},
	}
	c, err := NewContainerd(env)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = c.Execute(&buf, env)
	require.NoError(t, err)
	out := buf.String()

	// V2 template should use pkg_update instead of apt-get update
	assert.NotContains(t, out, "sudo apt-get update",
		"containerd v2 template should not use bare apt-get update")
	assert.Contains(t, out, "pkg_update",
		"containerd v2 template should use pkg_update abstraction")
}
