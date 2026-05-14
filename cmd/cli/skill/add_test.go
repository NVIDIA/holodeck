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
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v2"
)

// newAddContext builds a *cli.Context with the given positional args.
// The command's flag fields are populated by the caller before invocation.
func newAddContext(args []string) *cli.Context {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	_ = set.Parse(args)
	return cli.NewContext(app, set, nil)
}

func TestRunAdd_NoAgentFlag(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), out: &buf, skillName: "using-holodeck"}
	err := c.runAdd(newAddContext([]string{"using-holodeck"}))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one of --claude") {
		t.Errorf("error = %v, want it to mention agent flags", err)
	}
}

func TestRunAdd_NoNameNoAll(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), out: &buf, claude: true}
	err := c.runAdd(newAddContext(nil))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "skill name required") {
		t.Errorf("error = %v, want it to mention required name", err)
	}
}

func TestRunAdd_NameAndAll(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), out: &buf, claude: true, all: true}
	err := c.runAdd(newAddContext([]string{"using-holodeck"}))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %v, want mutual-exclusivity message", err)
	}
}

func TestRunAdd_UnknownSkill(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), out: &buf, claude: true}
	err := c.runAdd(newAddContext([]string{"no-such-skill"}))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not-found message", err)
	}
}

func TestRunAdd_StdoutMultiTarget(t *testing.T) {
	var buf bytes.Buffer
	c := &command{
		log: logger.NewLogger(), out: &buf,
		claude: true, cursor: true,
		stdout: true,
	}
	err := c.runAdd(newAddContext([]string{"using-holodeck"}))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--stdout requires") {
		t.Errorf("error = %v, want stdout-restriction message", err)
	}
}

func TestRunAdd_DryRun_ClaudeOnly(t *testing.T) {
	// Use --dry-run so no files are written, but the run path
	// exercises full resolution + per-renderer iteration.
	tmp := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	var buf bytes.Buffer
	c := &command{
		log: logger.NewLogger(), out: &buf,
		claude: true, dryRun: true,
	}
	if err := c.runAdd(newAddContext([]string{"using-holodeck"})); err != nil {
		t.Fatalf("runAdd: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "would write") {
		t.Errorf("dry-run output missing 'would write'; got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(".claude", "skills", "using-holodeck", "SKILL.md")) {
		t.Errorf("dry-run output missing expected path; got:\n%s", out)
	}
}

func TestRunAdd_AllAgents_SetsAllFour(t *testing.T) {
	tmp := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	var buf bytes.Buffer
	c := &command{
		log: logger.NewLogger(), out: &buf,
		allAgents: true, dryRun: true,
	}
	if err := c.runAdd(newAddContext([]string{"using-holodeck"})); err != nil {
		t.Fatalf("runAdd: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"SKILL.md", ".mdc", "AGENTS.md", "GEMINI.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected --all-agents to render for %s; got:\n%s", want, out)
		}
	}
}

// TestNewCommand_HasAddSubcommand verifies that NewCommand wires
// the `add` subcommand into the skill command. Catches: add subcommand
// dropped from build() or renamed.
func TestNewCommand_HasAddSubcommand(t *testing.T) {
	cmd := NewCommand(logger.NewLogger())
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}
	var hasAdd bool
	for _, sub := range cmd.Subcommands {
		if sub.Name == "add" {
			hasAdd = true
			break
		}
	}
	if !hasAdd {
		t.Errorf("missing 'add' subcommand in skill command")
	}
}
