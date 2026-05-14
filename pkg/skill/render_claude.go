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
	"fmt"
	"os"
	"path/filepath"
)

type claudeRenderer struct{}

// NewClaudeRenderer returns a Renderer for Claude Code.
func NewClaudeRenderer() Renderer { return &claudeRenderer{} }

func (claudeRenderer) AgentName() string { return "claude" }

func (claudeRenderer) SingleFile() bool { return false }

func (claudeRenderer) Render(s Skill) ([]byte, error) {
	out := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s",
		s.Name, s.Description, s.Body)
	// Ensure trailing newline for POSIX-friendliness.
	if out[len(out)-1] != '\n' {
		out += "\n"
	}
	return []byte(out), nil
}

func (claudeRenderer) InstallPath(s Skill, global bool) (string, error) {
	rel := filepath.Join(".claude", "skills", s.Name, "SKILL.md")
	if !global {
		return rel, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home dir: %w", err)
	}
	return filepath.Join(home, rel), nil
}
