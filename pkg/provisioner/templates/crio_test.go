package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewCriO(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.25",
			},
		},
	}
	crio := NewCriO(env)
	if crio.Version != "1.25" {
		t.Errorf("expected Version to be '1.25', got '%s'", crio.Version)
	}
}

func TestCriO_Execute(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Version: "1.25",
			},
		},
	}
	crio := NewCriO(env)
	var buf bytes.Buffer
	err := crio.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "apt install -y cri-o") {
		t.Errorf("template output missing cri-o install: %s", out)
	}
	if !strings.Contains(out, "systemctl start crio.service") {
		t.Errorf("template output missing crio start: %s", out)
	}

	// Test safe exit
	if !strings.Contains(out, "exit 0") {
		t.Errorf("template output missing safe exit: %s", out)
	}
}
