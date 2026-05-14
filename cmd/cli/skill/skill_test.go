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
	"testing"

	"github.com/NVIDIA/holodeck/internal/logger"
)

// TestNewCommand_HasListSubcommand verifies that the top-level
// skill command exposes the `list` subcommand with the correct
// name. Catches: list subcommand removed from build(), the parent
// command renamed away from "skill", or NewCommand returning nil.
//
// The `add` subcommand contract is verified in add_test.go once
// that subcommand is wired into build().
func TestNewCommand_HasListSubcommand(t *testing.T) {
	cmd := NewCommand(logger.NewLogger())
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}
	if cmd.Name != "skill" {
		t.Errorf("Name = %q, want %q", cmd.Name, "skill")
	}
	var hasList bool
	for _, sub := range cmd.Subcommands {
		if sub.Name == "list" {
			hasList = true
			break
		}
	}
	if !hasList {
		t.Errorf("missing 'list' subcommand in skill command")
	}
}
