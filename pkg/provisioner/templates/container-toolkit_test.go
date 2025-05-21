package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewContainerToolkit_Defaults(t *testing.T) {
	env := v1alpha1.Environment{}
	ctk := NewContainerToolkit(env)
	if ctk.ContainerRuntime != "containerd" {
		t.Errorf("expected default ContainerRuntime to be 'containerd', got '%s'", ctk.ContainerRuntime)
	}
	if ctk.EnableCDI != false {
		t.Errorf("expected default EnableCDI to be false, got %v", ctk.EnableCDI)
	}
}

func TestNewContainerToolkit_Custom(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "docker",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
			},
		},
	}
	ctk := NewContainerToolkit(env)
	if ctk.ContainerRuntime != "docker" {
		t.Errorf("expected ContainerRuntime to be 'docker', got '%s'", ctk.ContainerRuntime)
	}
	if ctk.EnableCDI != true {
		t.Errorf("expected EnableCDI to be true, got %v", ctk.EnableCDI)
	}
}

func TestContainerToolkit_Execute(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				EnableCDI: true,
			},
		},
	}
	ctk := NewContainerToolkit(env)
	var buf bytes.Buffer
	err := ctk.Execute(&buf, env)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "nvidia-ctk runtime configure --runtime=containerd --set-as-default --enable-cdi=true") {
		t.Errorf("template output missing expected runtime config: %s", out)
	}
}
