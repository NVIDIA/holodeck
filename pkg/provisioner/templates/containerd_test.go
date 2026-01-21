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

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewContainerd_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	c := NewContainerd(env)
	if c.Version != "1.7.27" {
		t.Errorf("expected default Version to be '1.7.27', got '%s'", c.Version)
	}
	if c.MajorVersion != 1 {
		t.Errorf("expected default MajorVersion to be 1, got %d", c.MajorVersion)
	}
}

func TestNewContainerd_CustomVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "v1.7.0",
			},
		},
	}
	c := NewContainerd(env)
	if c.Version != "1.7.0" {
		t.Errorf("expected Version to be '1.7.0', got '%s'", c.Version)
	}
	if c.MajorVersion != 1 {
		t.Errorf("expected MajorVersion to be 1, got %d", c.MajorVersion)
	}
}

func TestNewContainerd_EmptyVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "",
			},
		},
	}
	c := NewContainerd(env)
	if c.Version != "1.7.27" {
		t.Errorf("expected default Version to be '1.7.27' when empty, got '%s'", c.Version)
	}
	if c.MajorVersion != 1 {
		t.Errorf("expected default MajorVersion to be 1, got %d", c.MajorVersion)
	}
}

func TestNewContainerd_Version2(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "2.0.0",
			},
		},
	}
	c := NewContainerd(env)
	if c.Version != "2.0.0" {
		t.Errorf("expected Version to be '2.0.0', got '%s'", c.Version)
	}
	if c.MajorVersion != 2 {
		t.Errorf("expected MajorVersion to be 2, got %d", c.MajorVersion)
	}
}

func TestContainerd_Execute_Version1(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.7.26",
			},
		},
	}
	c := NewContainerd(env)
	var buf bytes.Buffer
	err := c.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()

	// Test idempotency framework
	if !strings.Contains(out, `COMPONENT="containerd"`) {
		t.Error("template output missing COMPONENT definition")
	}
	if !strings.Contains(out, "holodeck_progress") {
		t.Error("template output missing holodeck_progress calls")
	}

	// Test v1 template specifics - supports both apt and dnf/yum
	if !strings.Contains(out, "Installing containerd 1.7.26 using package repository") {
		t.Error("template output missing version installation message")
	}
	// Template now supports multiple package managers, test for version reference
	if !strings.Contains(out, "1.7.26") {
		t.Error("template output missing containerd version")
	}
	if !strings.Contains(out, "download.docker.com") {
		t.Error("template output missing Docker repository reference")
	}

	// Test Amazon Linux Fedora version mapping (P3 fix)
	// The template uses the HOLODECK_AMZN_FEDORA_VERSION variable set by CommonFunctions
	if !strings.Contains(out, "HOLODECK_AMZN_FEDORA_VERSION") {
		t.Error("template output missing Amazon Linux Fedora version variable reference")
	}
	// Verify we use the variable instead of hardcoded "39"
	if strings.Contains(out, `'s/\$releasever/39/g'`) {
		t.Error("template should use HOLODECK_AMZN_FEDORA_VERSION, not hardcoded 39")
	}

	// Test common configuration
	if !strings.Contains(out, "SystemdCgroup \\= true") {
		t.Error("template output missing SystemdCgroup configuration")
	}
	if !strings.Contains(out, "containerd config default") {
		t.Error("template output missing config generation")
	}

	// Test CNI path configuration fix
	if !strings.Contains(out, `conf_dir = "/etc/cni/net.d"`) {
		t.Error("template output missing CNI conf_dir configuration")
	}
	if !strings.Contains(out, `bin_dir = "/opt/cni/bin"`) {
		t.Error("template output missing CNI bin_dir configuration")
	}

	// Test verification
	if !strings.Contains(out, "holodeck_verify_containerd") {
		t.Error("template output missing containerd verification")
	}
	if !strings.Contains(out, "holodeck_mark_installed") {
		t.Error("template output missing mark installed call")
	}
}

func TestContainerd_Execute_Version2(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "2.0.0",
			},
		},
	}
	c := NewContainerd(env)
	var buf bytes.Buffer
	err := c.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()

	// Test idempotency framework
	if !strings.Contains(out, `COMPONENT="containerd"`) {
		t.Error("template output missing COMPONENT definition")
	}
	if !strings.Contains(out, "holodeck_progress") {
		t.Error("template output missing holodeck_progress calls")
	}

	// Test v2 template specifics - now using official binaries
	if !strings.Contains(out, "Installing containerd 2.0.0 from official binaries") {
		t.Error("template output missing v2 installation message")
	}
	if !strings.Contains(out, "containerd-2.0.0-linux-${ARCH}.tar.gz") {
		t.Error("template output missing containerd tarball name")
	}
	if !strings.Contains(out, "https://github.com/containerd/containerd/releases/download/v2.0.0/") {
		t.Error("template output missing containerd download URL")
	}

	// Test common configuration
	if !strings.Contains(out, "SystemdCgroup = true") {
		t.Error("template output missing SystemdCgroup configuration")
	}
	if !strings.Contains(out, "containerd config default") {
		t.Error("template output missing config generation")
	}

	// Test runc installation
	if !strings.Contains(out, "RUNC_VERSION=\"1.2.3\"") {
		t.Error("template output missing runc version")
	}

	// Test CNI plugins installation
	if !strings.Contains(out, "CNI_VERSION=\"v1.6.2\"") {
		t.Error("template output missing CNI version")
	}

	// Test CNI path configuration fix
	if !strings.Contains(out, `conf_dir = "/etc/cni/net.d"`) {
		t.Error("template output missing CNI conf_dir configuration")
	}
	if !strings.Contains(out, `bin_dir = "/opt/cni/bin"`) {
		t.Error("template output missing CNI bin_dir configuration")
	}

	// Test verification
	if !strings.Contains(out, "holodeck_verify_containerd") {
		t.Error("template output missing containerd verification")
	}
	if !strings.Contains(out, "holodeck_mark_installed") {
		t.Error("template output missing mark installed call")
	}
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
			c := NewContainerd(env)
			var buf bytes.Buffer
			err := c.Execute(&buf, env)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			out := buf.String()

			// Test kernel module handling (v2 has explicit modprobe, v1 relies on package manager)
			if tt.version == "2.0.0" {
				if !strings.Contains(out, "sudo modprobe overlay") {
					t.Error("template output missing overlay module loading")
				}
				if !strings.Contains(out, "sudo modprobe br_netfilter") {
					t.Error("template output missing br_netfilter module loading")
				}
			}

			// Test sysctl settings (v2 has explicit sysctl, v1 relies on package setup)
			if tt.version == "2.0.0" {
				if !strings.Contains(out, "net.bridge.bridge-nf-call-iptables") {
					t.Error("template output missing bridge-nf-call-iptables setting")
				}
				if !strings.Contains(out, "net.ipv4.ip_forward") {
					t.Error("template output missing ip_forward setting")
				}
				if !strings.Contains(out, "sudo sysctl --system") {
					t.Error("template output missing sysctl apply")
				}
			}

			// Test architecture check - only v2 has explicit arch detection
			if tt.version == "2.0.0" {
				if !strings.Contains(out, "if [[ \"$ARCH\" == \"x86_64\" ]]") {
					t.Error("template output missing x86_64 architecture check")
				}
				if !strings.Contains(out, "ARCH=\"amd64\"") {
					t.Error("template output missing x86_64 to amd64 mapping")
				}
				if !strings.Contains(out, "elif [[ \"$ARCH\" == \"aarch64\" ]]") {
					t.Error("template output missing aarch64 architecture check")
				}
				if !strings.Contains(out, "ARCH=\"arm64\"") {
					t.Error("template output missing aarch64 to arm64 mapping")
				}
			}

			// Test sudo usage in critical operations
			if !strings.Contains(out, "sudo mkdir -p /etc/containerd") {
				t.Error("template output missing sudo for directory creation")
			}
			// v1 uses restart then enable, v2 uses enable --now
			if tt.version == "1.7.26" {
				if !strings.Contains(out, "sudo systemctl restart containerd") {
					t.Error("template output missing sudo for containerd restart")
				}
			} else {
				if !strings.Contains(out, "sudo systemctl enable --now containerd") {
					t.Error("template output missing sudo for containerd enable --now")
				}
			}
			if !strings.Contains(out, "sudo systemctl enable") {
				t.Error("template output missing sudo for service enable")
			}

			// Test CNI installation - only v2 has explicit CNI installation
			if tt.version == "2.0.0" {
				if !strings.Contains(out, "CNI_VERSION=\"v1.6.2\"") {
					t.Error("template output missing CNI version")
				}
				if !strings.Contains(out, "/opt/cni/bin") {
					t.Error("template output missing CNI directory")
				}
			}
		})
	}
}
