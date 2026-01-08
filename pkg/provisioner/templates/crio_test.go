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

	// Test idempotency framework usage
	if !strings.Contains(out, `COMPONENT="crio"`) {
		t.Error("template output missing COMPONENT definition")
	}
	if !strings.Contains(out, "holodeck_progress") {
		t.Error("template output missing holodeck_progress calls")
	}

	// Test CRI-O installation
	if !strings.Contains(out, "apt-get install -y cri-o") {
		t.Errorf("template output missing cri-o install: %s", out)
	}
	if !strings.Contains(out, "systemctl start crio.service") {
		t.Errorf("template output missing crio start: %s", out)
	}

	// Test verification
	if !strings.Contains(out, "holodeck_verify_crio") {
		t.Error("template output missing crio verification")
	}
	if !strings.Contains(out, "holodeck_mark_installed") {
		t.Error("template output missing mark installed call")
	}
}
