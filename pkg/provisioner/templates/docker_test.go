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

	// Test Docker installation
	if !strings.Contains(out, "docker-ce=$DOCKER_VERSION") {
		t.Errorf("template output missing expected docker version install command: %s", out)
	}
	if !strings.Contains(out, ": ${DOCKER_VERSION:=20.10.7}") {
		t.Errorf("template output missing version assignment: %s", out)
	}
	if !strings.Contains(out, "systemctl enable docker") {
		t.Errorf("template output missing enable docker: %s", out)
	}

	// Test cri-dockerd installation
	if !strings.Contains(out, "CRI_DOCKERD_VERSION=\"0.3.17\"") {
		t.Errorf("template output missing cri-dockerd version: %s", out)
	}
	if !strings.Contains(out, "curl -L ${CRI_DOCKERD_URL} | sudo tar xzv -C /usr/local/bin --strip-components=1") {
		t.Errorf("template output missing cri-dockerd installation command: %s", out)
	}
	if !strings.Contains(out, "systemctl enable cri-docker.service") {
		t.Errorf("template output missing enable cri-docker service: %s", out)
	}
	if !strings.Contains(out, "systemctl enable cri-docker.socket") {
		t.Errorf("template output missing enable cri-docker socket: %s", out)
	}
	if !strings.Contains(out, "systemctl start cri-docker.service") {
		t.Errorf("template output missing start cri-docker service: %s", out)
	}
}
