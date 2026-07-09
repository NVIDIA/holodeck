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
	"regexp"
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

// TestNewCommand_DescriptionUsesFlagFirstExamples mirrors the same
// regression guard that add_test.go has for the add subcommand, but
// for the parent `skill` command's Description (shown by
// `holodeck skill --help`). urfave/cli/v2's default parser stops
// parsing flags after the first positional, so any example shown to
// users in the form `holodeck skill add <name> --claude` will fail
// with "must specify at least one of --claude/...". This test rejects
// any positional-then-agent-flag pattern in the Description.
func TestNewCommand_DescriptionUsesFlagFirstExamples(t *testing.T) {
	cmd := NewCommand(logger.NewLogger())
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}
	if cmd.Description == "" {
		t.Fatal("Description is empty; nothing to scan")
	}
	// Positional-then-flag pattern: skill name followed by an agent flag.
	// The skill name token may be any name in the catalog (today only
	// "using-holodeck", but match any kebab-case name for future-proofing).
	bad := regexp.MustCompile(`skill add [a-z0-9][a-z0-9-]* --(claude|cursor|codex|gemini|all-agents)`)
	if loc := bad.FindStringIndex(cmd.Description); loc != nil {
		excerpt := cmd.Description[loc[0]:loc[1]]
		t.Errorf("Description contains positional-then-flag example %q; urfave/cli/v2 will reject it. Use flag-first ordering (e.g. 'skill add --claude <name>').", excerpt)
	}
	// Sanity: at least one flag-first example must be present so we
	// don't accidentally pass by removing all examples.
	if !strings.Contains(cmd.Description, "--claude using-holodeck") {
		t.Errorf("Description is missing the canonical flag-first example 'skill add --claude using-holodeck'; the regression guard would silently pass with empty examples")
	}
}
