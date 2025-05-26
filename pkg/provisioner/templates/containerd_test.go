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
	if c.Version != "1.6.27" {
		t.Errorf("expected default Version to be '1.6.27', got '%s'", c.Version)
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
	if c.Version != "1.6.27" {
		t.Errorf("expected default Version to be '1.6.27' when empty, got '%s'", c.Version)
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
				Version: "1.6.27",
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

	// Test version detection
	if !strings.Contains(out, "MAJOR_VERSION=$(echo $CONTAINERD_VERSION | cut -d. -f1)") {
		t.Error("template output missing version detection")
	}

	// Test 1.x specific configuration
	if !strings.Contains(out, "version = 1") {
		t.Error("template output missing version 1 configuration")
	}
	if !strings.Contains(out, "runtime_type = \"io.containerd.runtime.v1.linux\"") {
		t.Error("template output missing 1.x runtime configuration")
	}

	// Test common configuration
	if !strings.Contains(out, "systemd_cgroup = true") {
		t.Error("template output missing systemd cgroup configuration")
	}
	if !strings.Contains(out, "sandbox_image = \"registry.k8s.io/pause:3.9\"") {
		t.Error("template output missing sandbox image configuration")
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

	// Test version detection
	if !strings.Contains(out, "MAJOR_VERSION=$(echo $CONTAINERD_VERSION | cut -d. -f1)") {
		t.Error("template output missing version detection")
	}

	// Test 2.x specific configuration
	if !strings.Contains(out, "version = 2") {
		t.Error("template output missing version 2 configuration")
	}
	if !strings.Contains(out, "runtime_type = \"io.containerd.runc.v2\"") {
		t.Error("template output missing 2.x runtime configuration")
	}

	// Test common configuration
	if !strings.Contains(out, "systemd_cgroup = true") {
		t.Error("template output missing systemd cgroup configuration")
	}
	if !strings.Contains(out, "sandbox_image = \"registry.k8s.io/pause:3.9\"") {
		t.Error("template output missing sandbox image configuration")
	}
}

func TestContainerd_Execute_SystemChecks(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.6.27",
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
