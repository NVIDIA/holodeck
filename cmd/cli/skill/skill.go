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

// Package skill exposes the `holodeck skill` CLI command, which lists
// and installs the embedded agentic skill catalog.
package skill

import (
	"io"
	"os"

	"github.com/NVIDIA/holodeck/internal/logger"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log *logger.FunLogger

	// list flags
	outputFormat string

	// add flags
	skillName string
	all       bool
	claude    bool
	cursor    bool
	codex     bool
	gemini    bool
	allAgents bool
	global    bool
	force     bool
	stdout    bool
	dryRun    bool

	// out is the writer used by the action methods; defaults to
	// os.Stdout. Tests override it to capture output.
	out io.Writer
}

// NewCommand constructs the skill command with the specified logger.
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := &command{log: log, out: os.Stdout}
	return c.build()
}

func (c *command) build() *cli.Command {
	return &cli.Command{
		Name:  "skill",
		Usage: "Manage holodeck agentic-skill catalog",
		Description: `Manage holodeck agentic skills.

Skills are short markdown documents that teach an AI coding agent
how to drive the holodeck CLI. They are embedded in the holodeck
binary and can be installed into Claude Code, Cursor, Codex, or
Gemini CLI in each agent's native format.

Examples:
  # Show the catalog
  holodeck skill list

  # Install one skill for Claude Code in the current project
  holodeck skill add --claude using-holodeck

  # Install for multiple agents
  holodeck skill add --claude --cursor using-holodeck

  # Install every skill for all four agents user-wide
  holodeck skill add --all --all-agents --global`,
		Subcommands: []*cli.Command{
			c.buildListCommand(),
			c.buildAddCommand(),
		},
	}
}
