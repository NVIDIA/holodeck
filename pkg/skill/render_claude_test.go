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

func TestClaudeRendererRender(t *testing.T) {
	r := &claudeRenderer{}
	s := Skill{
		Name:        "demo",
		Description: "A demo skill",
		Body:        "# Body\n\nDo the thing.\n",
	}
	got, err := r.Render(s)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "---\nname: demo\ndescription: A demo skill\n---\n\n# Body\n\nDo the thing.\n"
	if string(got) != want {
		t.Errorf("Render output mismatch\n got: %q\nwant: %q", string(got), want)
	}
}

func TestClaudeRendererAgentName(t *testing.T) {
	r := &claudeRenderer{}
	if r.AgentName() != "claude" {
		t.Errorf("AgentName = %q, want %q", r.AgentName(), "claude")
	}
}

func TestClaudeRendererSingleFile(t *testing.T) {
	r := &claudeRenderer{}
	if r.SingleFile() {
		t.Errorf("claude renderer should not be single-file")
	}
}

func TestClaudeRendererInstallPath(t *testing.T) {
	r := &claudeRenderer{}
	s := Skill{Name: "demo"}

	t.Run("project-local", func(t *testing.T) {
		path, err := r.InstallPath(s, false)
		if err != nil {
			t.Fatalf("InstallPath: %v", err)
		}
		want := filepath.Join(".claude", "skills", "demo", "SKILL.md")
		if path != want {
			t.Errorf("InstallPath = %q, want %q", path, want)
		}
	})

	t.Run("global", func(t *testing.T) {
		path, err := r.InstallPath(s, true)
		if err != nil {
			t.Fatalf("InstallPath: %v", err)
		}
		if !strings.HasSuffix(path, filepath.Join(".claude", "skills", "demo", "SKILL.md")) {
			t.Errorf("global InstallPath = %q, want suffix .claude/skills/demo/SKILL.md", path)
		}
		if !filepath.IsAbs(path) {
			t.Errorf("global InstallPath should be absolute; got %q", path)
		}
	})
}
