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

func TestCursorRendererRender(t *testing.T) {
	r := &cursorRenderer{}
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
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("output should start with frontmatter delimiter; got prefix %q",
			out[:minInt(len(out), 4)])
	}
	for _, want := range []string{
		"description: A demo skill",
		"globs: [\"**/*\"]",
		"alwaysApply: false",
		"# Body",
		"Do the thing.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "name:") {
		t.Errorf("cursor output should not include name field; got:\n%s", out)
	}
}

func TestCursorRendererAgentName(t *testing.T) {
	r := &cursorRenderer{}
	if r.AgentName() != "cursor" {
		t.Errorf("AgentName = %q, want %q", r.AgentName(), "cursor")
	}
}

func TestCursorRendererSingleFile(t *testing.T) {
	r := &cursorRenderer{}
	if r.SingleFile() {
		t.Errorf("cursor renderer should not be single-file")
	}
}

func TestCursorRendererInstallPath(t *testing.T) {
	r := &cursorRenderer{}
	s := Skill{Name: "demo"}

	t.Run("project-local", func(t *testing.T) {
		path, err := r.InstallPath(s, false)
		if err != nil {
			t.Fatalf("InstallPath: %v", err)
		}
		want := filepath.Join(".cursor", "rules", "demo.mdc")
		if path != want {
			t.Errorf("InstallPath = %q, want %q", path, want)
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
		if !strings.HasSuffix(path, filepath.Join(".cursor", "rules", "demo.mdc")) {
			t.Errorf("global InstallPath = %q, want suffix .cursor/rules/demo.mdc", path)
		}
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
