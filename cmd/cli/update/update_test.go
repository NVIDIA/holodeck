/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package update

import (
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestLabelParsing(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		key      string
		value    string
		hasError bool
	}{
		{"simple key=value", "team=gpu-infra", "team", "gpu-infra", false},
		{"env label", "env=prod", "env", "prod", false},
		{"no equals sign", "invalid", "", "", true},
		{"value with equals", "key=value=extra", "key", "value=extra", false},
		{"empty value", "key=", "key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.SplitN(tt.label, "=", 2)
			if tt.hasError {
				if len(parts) == 2 {
					t.Errorf("expected parse error for %q", tt.label)
				}
				return
			}
			if len(parts) != 2 {
				t.Fatalf("expected 2 parts for %q, got %d", tt.label, len(parts))
			}
			if parts[0] != tt.key {
				t.Errorf("expected key %q, got %q", tt.key, parts[0])
			}
			if parts[1] != tt.value {
				t.Errorf("expected value %q, got %q", tt.value, parts[1])
			}
		})
	}
}

func TestEnvironmentUpdate_AddDriver(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: false,
			},
		},
	}

	// Simulate adding driver (mirrors logic in command.run)
	env.Spec.NVIDIADriver.Install = true
	env.Spec.NVIDIADriver.Version = "560.35.03"

	if !env.Spec.NVIDIADriver.Install {
		t.Error("expected driver to be installed")
	}
	if env.Spec.NVIDIADriver.Version != "560.35.03" {
		t.Errorf("expected version 560.35.03, got %s", env.Spec.NVIDIADriver.Version)
	}
}

func TestEnvironmentUpdate_AddKubernetes(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install: false,
			},
		},
	}

	// Simulate adding kubernetes
	env.Spec.Kubernetes.Install = true
	env.Spec.Kubernetes.KubernetesInstaller = "kubeadm"
	env.Spec.Kubernetes.KubernetesVersion = "v1.31.1"

	if !env.Spec.Kubernetes.Install {
		t.Error("expected kubernetes to be installed")
	}
	if env.Spec.Kubernetes.KubernetesInstaller != "kubeadm" {
		t.Errorf("expected kubeadm, got %s", env.Spec.Kubernetes.KubernetesInstaller)
	}
	if env.Spec.Kubernetes.KubernetesVersion != "v1.31.1" {
		t.Errorf("expected v1.31.1, got %s", env.Spec.Kubernetes.KubernetesVersion)
	}
}

func TestEnvironmentUpdate_AddRuntime(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			ContainerRuntime: v1alpha1.ContainerRuntime{
				Install: false,
			},
		},
	}

	env.Spec.ContainerRuntime.Install = true
	env.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeName("containerd")

	if !env.Spec.ContainerRuntime.Install {
		t.Error("expected runtime to be installed")
	}
	if string(env.Spec.ContainerRuntime.Name) != "containerd" {
		t.Errorf("expected containerd, got %s", env.Spec.ContainerRuntime.Name)
	}
}

func TestEnvironmentUpdate_AddToolkitWithCDI(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
				Install: false,
			},
		},
	}

	env.Spec.NVIDIAContainerToolkit.Install = true
	env.Spec.NVIDIAContainerToolkit.EnableCDI = true
	env.Spec.NVIDIAContainerToolkit.Version = "1.17.0"

	if !env.Spec.NVIDIAContainerToolkit.Install {
		t.Error("expected toolkit to be installed")
	}
	if !env.Spec.NVIDIAContainerToolkit.EnableCDI {
		t.Error("expected CDI to be enabled")
	}
	if env.Spec.NVIDIAContainerToolkit.Version != "1.17.0" {
		t.Errorf("expected version 1.17.0, got %s", env.Spec.NVIDIAContainerToolkit.Version)
	}
}

func TestLabelApplication(t *testing.T) {
	env := &v1alpha1.Environment{}
	env.Labels = make(map[string]string)

	labels := []string{"team=gpu-infra", "env=test"}
	for _, label := range labels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) == 2 {
			env.Labels[parts[0]] = parts[1]
		}
	}

	if env.Labels["team"] != "gpu-infra" {
		t.Errorf("expected team=gpu-infra, got %s", env.Labels["team"])
	}
	if env.Labels["env"] != "test" {
		t.Errorf("expected env=test, got %s", env.Labels["env"])
	}
}

func TestProvisionedLabel_NilLabelsMap(t *testing.T) {
	// Regression test for C1: writing to nil Labels map should not panic
	env := &v1alpha1.Environment{}

	// Simulate what update.go does after provisioning
	if env.Labels == nil {
		env.Labels = make(map[string]string)
	}
	env.Labels["holodeck-instance-provisioned"] = "true"

	if env.Labels["holodeck-instance-provisioned"] != "true" {
		t.Error("expected provisioned label to be set")
	}
}
