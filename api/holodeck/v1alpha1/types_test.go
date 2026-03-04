/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package v1alpha1

import (
	"encoding/json"
	"testing"
)

func TestCustomTemplate_JSONRoundTrip(t *testing.T) {
	tpl := CustomTemplate{
		Name:   "install-monitoring",
		Phase:  TemplatePhasePostKubernetes,
		Inline: "#!/bin/bash\necho hello",
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got CustomTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Name != tpl.Name {
		t.Errorf("Name: got %q, want %q", got.Name, tpl.Name)
	}
	if got.Phase != tpl.Phase {
		t.Errorf("Phase: got %q, want %q", got.Phase, tpl.Phase)
	}
	if got.Inline != tpl.Inline {
		t.Errorf("Inline: got %q, want %q", got.Inline, tpl.Inline)
	}
}

func TestCustomTemplate_AllSources(t *testing.T) {
	tests := []struct {
		name string
		tpl  CustomTemplate
	}{
		{
			name: "inline source",
			tpl: CustomTemplate{
				Name:   "inline-test",
				Inline: "echo hello",
			},
		},
		{
			name: "file source",
			tpl: CustomTemplate{
				Name: "file-test",
				File: "./scripts/test.sh",
			},
		},
		{
			name: "url source",
			tpl: CustomTemplate{
				Name:     "url-test",
				URL:      "https://example.com/script.sh",
				Checksum: "sha256:abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.tpl)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			var got CustomTemplate
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got.Name != tt.tpl.Name {
				t.Errorf("Name: got %q, want %q", got.Name, tt.tpl.Name)
			}
		})
	}
}

func TestTemplatePhase_Constants(t *testing.T) {
	phases := []TemplatePhase{
		TemplatePhasePreInstall,
		TemplatePhasePostRuntime,
		TemplatePhasePostToolkit,
		TemplatePhasePostKubernetes,
		TemplatePhasePostInstall,
	}
	expected := []string{
		"pre-install",
		"post-runtime",
		"post-toolkit",
		"post-kubernetes",
		"post-install",
	}
	for i, phase := range phases {
		if string(phase) != expected[i] {
			t.Errorf("phase %d: got %q, want %q", i, phase, expected[i])
		}
	}
}

func TestEnvironmentSpec_CustomTemplatesField(t *testing.T) {
	spec := EnvironmentSpec{
		CustomTemplates: []CustomTemplate{
			{Name: "test", Phase: TemplatePhasePostInstall, Inline: "echo ok"},
		},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got EnvironmentSpec
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(got.CustomTemplates) != 1 {
		t.Fatalf("expected 1 custom template, got %d", len(got.CustomTemplates))
	}
	if got.CustomTemplates[0].Name != "test" {
		t.Errorf("Name: got %q, want %q", got.CustomTemplates[0].Name, "test")
	}
}
