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
	if c.Version != "1.7.28" {
		t.Errorf("expected default Version to be '1.7.28', got '%s'", c.Version)
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
	if c.Version != "1.7.28" {
		t.Errorf("expected default Version to be '1.7.28' when empty, got '%s'", c.Version)
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

	// Test unified configuration (we now use version = 2 for all containerd versions)
	if !strings.Contains(out, "version = 2") {
		t.Error("template output missing version 2 configuration")
	}
	if !strings.Contains(out, "runtime_type = \"io.containerd.runc.v2\"") {
		t.Error("template output missing runc v2 runtime configuration")
	}

	// Test common configuration
	if !strings.Contains(out, "SystemdCgroup = true") {
		t.Error("template output missing SystemdCgroup configuration")
	}
	if !strings.Contains(out, "sandbox_image = \"registry.k8s.io/pause:3.9\"") {
		t.Error("template output missing sandbox image configuration")
	}

	// Test CNI configuration
	if !strings.Contains(out, "bin_dir = \"/opt/cni/bin:/usr/libexec/cni\"") {
		t.Error("template output missing CNI bin_dir configuration")
	}
	if !strings.Contains(out, "conf_dir = \"/etc/cni/net.d\"") {
		t.Error("template output missing CNI conf_dir configuration")
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

	// Test unified configuration (we now use version = 2 for all containerd versions)
	if !strings.Contains(out, "version = 2") {
		t.Error("template output missing version 2 configuration")
	}
	if !strings.Contains(out, "runtime_type = \"io.containerd.runc.v2\"") {
		t.Error("template output missing runc v2 runtime configuration")
	}

	// Test common configuration
	if !strings.Contains(out, "SystemdCgroup = true") {
		t.Error("template output missing SystemdCgroup configuration")
	}
	if !strings.Contains(out, "sandbox_image = \"registry.k8s.io/pause:3.9\"") {
		t.Error("template output missing sandbox image configuration")
	}

	// Test CNI configuration
	if !strings.Contains(out, "bin_dir = \"/opt/cni/bin:/usr/libexec/cni\"") {
		t.Error("template output missing CNI bin_dir configuration")
	}
	if !strings.Contains(out, "conf_dir = \"/etc/cni/net.d\"") {
		t.Error("template output missing CNI conf_dir configuration")
	}
}

func TestContainerd_Execute_SystemChecks(t *testing.T) {
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

	// Test systemd check
	if !strings.Contains(out, "if ! command -v systemctl &> /dev/null") {
		t.Error("template output missing systemd check")
	}

	// Test kernel module handling
	if !strings.Contains(out, "REQUIRED_MODULES=\"overlay br_netfilter\"") {
		t.Error("template output missing required modules check")
	}
	if !strings.Contains(out, "sudo modprobe ${module}") {
		t.Error("template output missing sudo module loading")
	}
	if !strings.Contains(out, "/etc/modules-load.d/${module}.conf") {
		t.Error("template output missing module persistence")
	}

	// Test sysctl settings
	if !strings.Contains(out, "sudo sysctl -n $key") {
		t.Error("template output missing sudo sysctl check")
	}
	if !strings.Contains(out, "sudo sysctl -w $key=$value") {
		t.Error("template output missing sudo sysctl setting")
	}
	if !strings.Contains(out, "sudo sysctl --system") {
		t.Error("template output missing sudo sysctl apply")
	}

	// Test architecture check
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
	if !strings.Contains(out, "else") {
		t.Error("template output missing else clause for unsupported architectures")
	}

	// Test temporary directory handling
	if !strings.Contains(out, "TMP_DIR=$(mktemp -d)") {
		t.Error("template output missing temporary directory creation")
	}

	// Test error handling
	if !strings.Contains(out, "Error: Failed to download containerd tarball") {
		t.Error("template output missing download error handling")
	}
	if !strings.Contains(out, "Error: Checksum verification failed for containerd") {
		t.Error("template output missing checksum error handling")
	}

	// Test sudo usage in critical operations
	if !strings.Contains(out, "sudo mkdir -p /etc/containerd") {
		t.Error("template output missing sudo for directory creation")
	}
	if !strings.Contains(out, "sudo systemctl daemon-reload") {
		t.Error("template output missing sudo for systemd reload")
	}
	if !strings.Contains(out, "sudo systemctl enable --now containerd") {
		t.Error("template output missing sudo for service enable")
	}
}
