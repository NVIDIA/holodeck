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
	"strings"

	pkgskill "github.com/NVIDIA/holodeck/pkg/skill"

	cli "github.com/urfave/cli/v2"
)

func (c *command) buildAddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Install one or more skills for one or more agents",
		ArgsUsage: "[skill-name]",
		Description: `Install a skill from the embedded catalog into one or more agents.

Targets: --claude, --cursor, --codex, --gemini (or --all-agents).
Skill selection: positional <skill-name>, or --all for every skill.

Examples:
  holodeck skill add --claude using-holodeck
  holodeck skill add --claude --cursor --global using-holodeck
  holodeck skill add --all --all-agents`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "claude", Usage: "install for Claude Code", Destination: &c.claude},
			&cli.BoolFlag{Name: "cursor", Usage: "install for Cursor IDE", Destination: &c.cursor},
			&cli.BoolFlag{Name: "codex", Usage: "install for Codex CLI", Destination: &c.codex},
			&cli.BoolFlag{Name: "gemini", Usage: "install for Gemini CLI", Destination: &c.gemini},
			&cli.BoolFlag{Name: "all-agents", Usage: "install for all four agents", Destination: &c.allAgents},
			&cli.BoolFlag{Name: "all", Usage: "install every skill in the catalog (mutually exclusive with positional name)", Destination: &c.all},
			&cli.BoolFlag{Name: "global", Usage: "write to the user-wide config dir instead of CWD", Destination: &c.global},
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing per-file installs without prompting", Destination: &c.force},
			&cli.BoolFlag{Name: "stdout", Usage: "print rendered output instead of writing (requires exactly one skill and one agent)", Destination: &c.stdout},
			&cli.BoolFlag{Name: "dry-run", Usage: "print install paths without writing", Destination: &c.dryRun},
		},
		Action: c.runAdd,
	}
}

func (c *command) runAdd(ctx *cli.Context) error {
	renderers, err := c.selectedRenderers()
	if err != nil {
		return err
	}
	if len(renderers) == 0 {
		return fmt.Errorf("must specify at least one of --claude/--cursor/--codex/--gemini/--all-agents")
	}

	catalog, err := pkgskill.Catalog()
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	skills, err := c.selectedSkills(ctx, catalog)
	if err != nil {
		return err
	}

	if c.stdout && (len(skills) != 1 || len(renderers) != 1) {
		return fmt.Errorf("--stdout requires exactly one skill and one agent (got %d skills, %d agents)",
			len(skills), len(renderers))
	}

	for _, s := range skills {
		for _, r := range renderers {
			if c.stdout {
				rendered, err := r.Render(s)
				if err != nil {
					return err
				}
				if _, err := c.out.Write(rendered); err != nil {
					return err
				}
				continue
			}
			path, err := pkgskill.Install(r, s, pkgskill.InstallOptions{
				Global: c.global,
				Force:  c.force,
				DryRun: c.dryRun,
				Stdout: c.out,
			})
			if err != nil {
				return err
			}
			if !c.dryRun {
				if _, err := fmt.Fprintf(c.out, "installed %s for %s -> %s\n", s.Name, r.AgentName(), path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *command) selectedRenderers() ([]pkgskill.Renderer, error) {
	if c.allAgents {
		c.claude = true
		c.cursor = true
		c.codex = true
		c.gemini = true
	}
	var rs []pkgskill.Renderer
	if c.claude {
		rs = append(rs, pkgskill.NewClaudeRenderer())
	}
	if c.cursor {
		rs = append(rs, pkgskill.NewCursorRenderer())
	}
	if c.codex {
		rs = append(rs, pkgskill.NewCodexRenderer())
	}
	if c.gemini {
		rs = append(rs, pkgskill.NewGeminiRenderer())
	}
	return rs, nil
}

func (c *command) selectedSkills(ctx *cli.Context, catalog []pkgskill.Skill) ([]pkgskill.Skill, error) {
	name := ""
	if ctx.NArg() > 0 {
		name = ctx.Args().First()
	}
	if c.all && name != "" {
		return nil, fmt.Errorf("--all is mutually exclusive with a skill name")
	}
	if !c.all && name == "" {
		return nil, fmt.Errorf("skill name required (or pass --all); available: %s",
			strings.Join(catalogNames(catalog), ", "))
	}
	if c.all {
		return catalog, nil
	}
	for _, s := range catalog {
		if s.Name == name {
			return []pkgskill.Skill{s}, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found; available: %s",
		name, strings.Join(catalogNames(catalog), ", "))
}

func catalogNames(catalog []pkgskill.Skill) []string {
	names := make([]string, len(catalog))
	for i, s := range catalog {
		names[i] = s.Name
	}
	return names
}
