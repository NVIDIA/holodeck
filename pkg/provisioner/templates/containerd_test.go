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

func TestContainerd_Execute(t *testing.T) {
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
	if !strings.Contains(out, "sed -i 's/SystemdCgroup \\= false/SystemdCgroup \\= true/g' /etc/containerd/config.toml") {
		t.Errorf("template output missing cgroup sed command: %s", out)
	}
}
