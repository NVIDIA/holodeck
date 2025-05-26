package provisioner

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestDryrun_ValidKubernetesVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install:             true,
				KubernetesInstaller: "kubeadm",
				KubernetesVersion:   "v1.20.0",
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed: %v", err)
	}
}

func TestDryrun_InvalidKubernetesVersion(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install:             true,
				KubernetesInstaller: "kubeadm",
				KubernetesVersion:   "1.20.0",
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err == nil {
		t.Error("Dryrun did not fail with invalid kubernetes version")
	}
}

func TestDryrun_ValidContainerRuntime(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    "containerd",
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err != nil {
		t.Errorf("Dryrun failed: %v", err)
	}
}

func TestDryrun_InvalidContainerRuntime(t *testing.T) {
	env := v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: true,
				Name:    "invalid",
			},
		},
	}
	log := logger.NewLogger()
	err := Dryrun(log, env)
	if err == nil {
		t.Error("Dryrun did not fail with invalid container runtime")
	}
}
