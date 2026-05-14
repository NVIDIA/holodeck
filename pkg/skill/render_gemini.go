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

type geminiRenderer struct{}

// NewGeminiRenderer returns a Renderer for the Gemini CLI.
func NewGeminiRenderer() Renderer { return &geminiRenderer{} }

func (geminiRenderer) AgentName() string { return "gemini" }

func (geminiRenderer) SingleFile() bool { return true }

func (geminiRenderer) Render(s Skill) ([]byte, error) {
	return []byte(renderSection(s)), nil
}

func (geminiRenderer) InstallPath(_ Skill, global bool) (string, error) {
	if !global {
		return "GEMINI.md", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home dir: %w", err)
	}
	return filepath.Join(home, ".gemini", "GEMINI.md"), nil
}
