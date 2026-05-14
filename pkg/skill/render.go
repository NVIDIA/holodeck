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

// Renderer translates a Skill into an agent-specific representation
// and reports where it should be installed.
//
// For per-file agents (claude, cursor), Render returns the full file
// contents and InstallPath returns the per-skill file path.
// For single-file agents (codex, gemini), Render returns a section
// block delimited by holodeck-skill markers and InstallPath returns
// the shared file path (independent of the skill name).
type Renderer interface {
	// AgentName returns the short identifier for the agent ("claude",
	// "cursor", "codex", "gemini").
	AgentName() string

	// Render produces the bytes to install for this skill.
	Render(Skill) ([]byte, error)

	// InstallPath returns the destination path. global=true selects
	// the user-wide install location; global=false selects project-
	// local (relative to CWD).
	InstallPath(skill Skill, global bool) (string, error)

	// SingleFile reports whether this agent writes into a shared file
	// (true) or a dedicated per-skill file (false).
	SingleFile() bool
}
