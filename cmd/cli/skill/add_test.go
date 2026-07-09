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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v3"
)

// newAddCommand builds a *cli.Command whose positional Args() are the given
// args. urfave/cli v3 has no exported constructor for a parsed Command, so we
// run a throwaway root and capture the sub-command from its Action. The
// command's flag fields are populated by the caller directly on the command
// struct before invocation, so this helper only needs to carry the positionals.
func newAddCommand(t *testing.T, args []string) *cli.Command {
	t.Helper()
	var captured *cli.Command
	root := &cli.Command{
		Name: "holodeck",
		Commands: []*cli.Command{
			{
				Name: "add",
				Action: func(_ context.Context, cmd *cli.Command) error {
					captured = cmd
					return nil
				},
			},
		},
	}
	if err := root.Run(context.Background(), append([]string{"holodeck", "add"}, args...)); err != nil {
		t.Fatalf("building test command with args %v: %v", args, err)
	}
	return captured
}

func TestRunAdd_NoAgentFlag(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), out: &buf, skillName: "using-holodeck"}
	err := c.runAdd(context.Background(), newAddCommand(t, []string{"using-holodeck"}))
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
	err := c.runAdd(context.Background(), newAddCommand(t, nil))
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
	err := c.runAdd(context.Background(), newAddCommand(t, []string{"using-holodeck"}))
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
	err := c.runAdd(context.Background(), newAddCommand(t, []string{"no-such-skill"}))
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
	err := c.runAdd(context.Background(), newAddCommand(t, []string{"using-holodeck"}))
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
	if err := c.runAdd(context.Background(), newAddCommand(t, []string{"using-holodeck"})); err != nil {
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
	if err := c.runAdd(context.Background(), newAddCommand(t, []string{"using-holodeck"})); err != nil {
		t.Fatalf("runAdd: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"SKILL.md", ".mdc", "AGENTS.md", "GEMINI.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected --all-agents to render for %s; got:\n%s", want, out)
		}
	}
}

// TestBuildAddCommand_DescriptionUsesFlagFirstExamples guards against
// docs regression: urfave/cli/v2's default parser stops parsing flags
// after the first non-flag arg, so 'skill add using-holodeck --claude'
// is rejected with "must specify at least one of --claude/...". The
// in-binary --help text MUST therefore show flag-first ordering.
func TestBuildAddCommand_DescriptionUsesFlagFirstExamples(t *testing.T) {
	c := &command{log: logger.NewLogger()}
	cmd := c.buildAddCommand()
	desc := cmd.Description
	if !strings.Contains(desc, "--claude using-holodeck") {
		t.Errorf("Description missing flag-first example %q; got:\n%s",
			"--claude using-holodeck", desc)
	}
	if strings.Contains(desc, "using-holodeck --claude") {
		t.Errorf("Description still contains broken positional-first example "+
			"%q; the urfave/cli/v2 parser rejects this ordering. Description:\n%s",
			"using-holodeck --claude", desc)
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
	for _, sub := range cmd.Commands {
		if sub.Name == "add" {
			hasAdd = true
			break
		}
	}
	if !hasAdd {
		t.Errorf("missing 'add' subcommand in skill command")
	}
}
