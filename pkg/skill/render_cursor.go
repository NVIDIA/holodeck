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

type cursorRenderer struct{}

// NewCursorRenderer returns a Renderer for Cursor IDE.
func NewCursorRenderer() Renderer { return &cursorRenderer{} }

func (cursorRenderer) AgentName() string { return "cursor" }

func (cursorRenderer) SingleFile() bool { return false }

func (cursorRenderer) Render(s Skill) ([]byte, error) {
	out := fmt.Sprintf("---\ndescription: %s\nglobs: [\"**/*\"]\nalwaysApply: false\n---\n\n%s",
		s.Description, s.Body)
	if out[len(out)-1] != '\n' {
		out += "\n"
	}
	return []byte(out), nil
}

func (cursorRenderer) InstallPath(s Skill, global bool) (string, error) {
	rel := filepath.Join(".cursor", "rules", s.Name+".mdc")
	if !global {
		return rel, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home dir: %w", err)
	}
	return filepath.Join(home, rel), nil
}
