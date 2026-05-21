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

package skill

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiRendererRender(t *testing.T) {
	r := &geminiRenderer{}
	s := Skill{
		Name:        "demo",
		Description: "A demo skill",
		Body:        "# Body\n\nDo the thing.\n",
	}
	got, err := r.Render(s)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)
	for _, want := range []string{
		"<!-- BEGIN holodeck-skill:demo -->",
		"<!-- END holodeck-skill:demo -->",
		"## demo",
		"# Body",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestGeminiRendererAgentName(t *testing.T) {
	r := &geminiRenderer{}
	if r.AgentName() != "gemini" {
		t.Errorf("AgentName = %q, want %q", r.AgentName(), "gemini")
	}
}

func TestGeminiRendererSingleFile(t *testing.T) {
	r := &geminiRenderer{}
	if !r.SingleFile() {
		t.Errorf("gemini renderer should be single-file")
	}
}

func TestGeminiRendererInstallPath(t *testing.T) {
	r := &geminiRenderer{}
	s := Skill{Name: "demo"}

	t.Run("project-local", func(t *testing.T) {
		path, err := r.InstallPath(s, false)
		if err != nil {
			t.Fatalf("InstallPath: %v", err)
		}
		if path != "GEMINI.md" {
			t.Errorf("InstallPath = %q, want %q", path, "GEMINI.md")
		}
	})

	t.Run("global", func(t *testing.T) {
		path, err := r.InstallPath(s, true)
		if err != nil {
			t.Fatalf("InstallPath: %v", err)
		}
		if !filepath.IsAbs(path) {
			t.Errorf("global InstallPath should be absolute; got %q", path)
		}
		if !strings.HasSuffix(path, filepath.Join(".gemini", "GEMINI.md")) {
			t.Errorf("global InstallPath = %q, want suffix .gemini/GEMINI.md", path)
		}
	})
}
