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

import "testing"

// TestRendererInterfaceSatisfied verifies the Claude renderer
// satisfies the Renderer interface contract: non-empty AgentName,
// Render returns non-empty bytes for valid input. Catches:
// AgentName returning empty string, Render returning empty bytes
// or an error for a valid skill, or a method removed from the
// interface (compile-time failure).
//
// Other renderers' interface conformance is verified in their own
// _test.go files.
var _ Renderer = NewClaudeRenderer() // compile-time interface check

func TestRendererInterfaceSatisfied(t *testing.T) {
	r := NewClaudeRenderer()
	if r.AgentName() == "" {
		t.Errorf("AgentName returned empty string")
	}
	out, err := r.Render(Skill{Name: "demo", Description: "d", Body: "b\n"})
	if err != nil {
		t.Errorf("Render: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("Render returned empty bytes")
	}
}
