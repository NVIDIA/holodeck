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

package main

import (
	"context"
	"os"
	"testing"

	"github.com/NVIDIA/holodeck/cmd/cli/ssh"
	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v3"
)

// TestInterspersedFlags_SkillAddNaturalOrder is the #813 regression: flags and
// positionals must interleave, so `holodeck skill add using-holodeck --claude`
// (positional before flags) parses instead of being rejected. urfave/cli v2's
// default parser stops at the first positional, so this FAILS under v2.
func TestInterspersedFlags_SkillAddNaturalOrder(t *testing.T) {
	chdirTemp(t)

	log := logger.NewLogger()
	app := NewApp(log)

	// --dry-run so nothing is written; we assert only that parsing succeeds.
	err := app.Run(context.Background(), []string{"holodeck", "skill", "add", "using-holodeck", "--claude", "--dry-run"})
	if err != nil {
		t.Fatalf("natural-order `skill add using-holodeck --claude --dry-run` should parse, got error: %v", err)
	}
}

// TestInterspersedFlags_SkillAddFlagFirst guards the flag-first ordering, which
// already works under v2 and must keep working after the migration.
func TestInterspersedFlags_SkillAddFlagFirst(t *testing.T) {
	chdirTemp(t)

	log := logger.NewLogger()
	app := NewApp(log)

	err := app.Run(context.Background(), []string{"holodeck", "skill", "add", "--claude", "--dry-run", "using-holodeck"})
	if err != nil {
		t.Fatalf("flag-first `skill add --claude --dry-run using-holodeck` should parse, got error: %v", err)
	}
}

// TestInterspersedFlags_SSHPassthroughPreservesRemoteFlags proves that the `--`
// terminator still hands the remote command (including its own flags) through to
// ssh untouched: `holodeck ssh <id> -- kubectl get nodes --remote-flag` must keep
// --remote-flag in the remote args rather than parsing it as an ssh flag.
//
// The real ssh command's Action does live I/O (instance lookup), so we inject a
// capturing Action onto the real command object to observe the args-parsing
// boundary without refactoring ssh internals.
func TestInterspersedFlags_SSHPassthroughPreservesRemoteFlags(t *testing.T) {
	log := logger.NewLogger()
	sshCmd := ssh.NewCommand(log)

	var captured []string
	sshCmd.Action = func(_ context.Context, c *cli.Command) error {
		captured = c.Args().Slice()
		return nil
	}

	app := &cli.Command{Name: "holodeck", Commands: []*cli.Command{sshCmd}}

	argv := []string{"holodeck", "ssh", "abc123", "--", "kubectl", "get", "nodes", "--remote-flag"}
	if err := app.Run(context.Background(), argv); err != nil {
		t.Fatalf("ssh passthrough parse failed (--remote-flag likely consumed as an ssh flag): %v", err)
	}

	remote := []string{"kubectl", "get", "nodes", "--remote-flag"}
	if !containsSubsequence(captured, remote) {
		t.Fatalf("expected remote command %v preserved contiguously in parsed args, got %v", remote, captured)
	}
}

// chdirTemp switches to a throwaway working directory for the duration of the
// test so dry-run path computation never touches the repo tree.
func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
}

// containsSubsequence reports whether want appears as a contiguous run in got.
func containsSubsequence(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for i := 0; i+len(want) <= len(got); i++ {
		match := true
		for j := range want {
			if got[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
