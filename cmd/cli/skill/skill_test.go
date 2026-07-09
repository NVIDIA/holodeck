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
	"strings"
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
	for _, sub := range cmd.Commands {
		if sub.Name == "list" {
			hasList = true
			break
		}
	}
	if !hasList {
		t.Errorf("missing 'list' subcommand in skill command")
	}
}

// TestNewCommand_MainGoWiringContract verifies the full surface that
// cmd/cli/main.go consumes when registering the skill command:
// non-nil, Name == "skill", non-empty Usage (renders in `--help`),
// and both `list` and `add` subcommands present.
//
// Failure modes caught:
//   - NewCommand returning nil (main.go would deref-panic on Run)
//   - Renaming the command away from "skill" (breaks docs and skill
//     install paths)
//   - Empty Usage (urfave/cli renders a blank line in the COMMANDS
//     section of the top-level help)
//   - Either subcommand silently dropped from build()
func TestNewCommand_MainGoWiringContract(t *testing.T) {
	cmd := NewCommand(logger.NewLogger())
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}
	if cmd.Name != "skill" {
		t.Errorf("Name = %q, want %q", cmd.Name, "skill")
	}
	if cmd.Usage == "" {
		t.Errorf("Usage is empty; the top-level help would render a blank command entry")
	}
	want := map[string]bool{"list": false, "add": false}
	for _, sub := range cmd.Commands {
		if _, ok := want[sub.Name]; ok {
			want[sub.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing %q subcommand on the skill command", name)
		}
	}
}

// TestNewCommand_DescriptionUsesNaturalOrderExamples mirrors the natural-order
// docs guard that add_test.go has for the add subcommand, but for the parent
// `skill` command's Description (shown by `holodeck skill --help`). Now that v3
// parses interspersed flags (#813), examples use natural (positional-first)
// ordering. Reverting the example to the old flag-first form
// "skill add --claude using-holodeck" turns this RED.
func TestNewCommand_DescriptionUsesNaturalOrderExamples(t *testing.T) {
	cmd := NewCommand(logger.NewLogger())
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}
	if !strings.Contains(cmd.Description, "skill add using-holodeck --claude") {
		t.Errorf("Description missing natural-order example %q; got:\n%s",
			"skill add using-holodeck --claude", cmd.Description)
	}
}
