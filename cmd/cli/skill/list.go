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
	"encoding/json"
	"fmt"
	"text/tabwriter"

	pkgskill "github.com/NVIDIA/holodeck/pkg/skill"

	cli "github.com/urfave/cli/v2"
	sigsyaml "sigs.k8s.io/yaml"
)

// SkillListItem represents one row of the catalog output.
type SkillListItem struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// SkillList wraps the catalog rows for table/json/yaml output.
type SkillList struct {
	Skills []SkillListItem `json:"skills" yaml:"skills"`
}

func (c *command) buildListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available skills in the embedded catalog",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "Output format: table, json, yaml (default: table)",
				Destination: &c.outputFormat,
				Value:       "table",
			},
		},
		Action: c.runList,
	}
}

func (c *command) runList(_ *cli.Context) error {
	skills, err := pkgskill.Catalog()
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	data := SkillList{Skills: make([]SkillListItem, 0, len(skills))}
	for _, s := range skills {
		data.Skills = append(data.Skills, SkillListItem{Name: s.Name, Description: s.Description})
	}

	switch c.outputFormat {
	case "", "table":
		tw := tabwriter.NewWriter(c.out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tDESCRIPTION")
		for _, s := range data.Skills {
			fmt.Fprintf(tw, "%s\t%s\n", s.Name, s.Description)
		}
		return tw.Flush()
	case "json":
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case "yaml":
		b, err := sigsyaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshalling yaml: %w", err)
		}
		_, err = c.out.Write(b)
		return err
	default:
		return fmt.Errorf("invalid output format %q: must be table, json, or yaml", c.outputFormat)
	}
}
