package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewDocker_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	d := NewDocker(env)
	if d.Version != "latest" {
		t.Errorf("expected default Version to be 'latest', got '%s'", d.Version)
	}
}

func TestNewDocker_CustomVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "20.10.7",
			},
		},
	}
	d := NewDocker(env)
	if d.Version != "20.10.7" {
		t.Errorf("expected Version to be '20.10.7', got '%s'", d.Version)
	}
}

func TestDocker_Execute(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "20.10.7",
			},
		},
	}
	d := NewDocker(env)
	var buf bytes.Buffer
	err := d.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "docker-ce=$DOCKER_VERSION") {
		t.Errorf("template output missing expected docker version install command: %s", out)
	}
	if !strings.Contains(out, ": ${DOCKER_VERSION:=20.10.7}") {
		t.Errorf("template output missing version assignment: %s", out)
	}
	if !strings.Contains(out, "systemctl enable docker") {
		t.Errorf("template output missing enable docker: %s", out)
	}
}
