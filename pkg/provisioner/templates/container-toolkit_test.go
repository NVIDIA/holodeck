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

	// Test CNI path verification after nvidia-ctk
	if !strings.Contains(out, "Verifying CNI configuration after nvidia-ctk") {
		t.Error("template output missing CNI verification message")
	}
	if !strings.Contains(out, `bin_dir = "/opt/cni/bin"`) {
		t.Error("template output missing correct CNI bin_dir check")
	}
	// Should NOT contain the old path with /usr/libexec/cni
	if strings.Contains(out, "/usr/libexec/cni") {
		t.Error("template output should not contain /usr/libexec/cni path")
	}
}
