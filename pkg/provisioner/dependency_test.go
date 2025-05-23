package provisioner

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNewDependencies(t *testing.T) {
	env := v1alpha1.Environment{}
	d := NewDependencies(env)
	if d == nil { //nolint:staticcheck
		t.Fatal("NewDependencies returned nil")
	}
	if len(d.Dependencies) != 0 { //nolint:staticcheck
		t.Errorf("expected empty dependencies, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_WithKubernetes(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Installer: "kubeadm",
			},
		},
	}
	d := NewDependencies(env)
	d.withKubernetes()
	if len(d.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_WithContainerRuntime(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Name: "containerd",
			},
		},
	}
	d := NewDependencies(env)
	d.withContainerRuntime()
	if len(d.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_WithContainerToolkit(t *testing.T) {
	env := v1alpha1.Environment{}
	d := NewDependencies(env)
	d.withContainerToolkit()
	if len(d.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_WithNVDriver(t *testing.T) {
	env := v1alpha1.Environment{}
	d := NewDependencies(env)
	d.withNVDriver()
	if len(d.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_WithKernel(t *testing.T) {
	env := v1alpha1.Environment{}
	d := NewDependencies(env)
	d.withKernel()
	if len(d.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(d.Dependencies))
	}
}

func TestDependencyResolver_Resolve(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install:   true,
				Installer: "kubeadm",
			},
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    "containerd",
			},
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: true,
			},
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: true,
			},
			Kernel: v1alpha1.Kernel{
				Version: "5.4.0",
			},
		},
	}
	d := NewDependencies(env)
	deps := d.Resolve()
	if len(deps) != 5 {
		t.Errorf("expected 5 dependencies, got %d", len(deps))
	}
}
