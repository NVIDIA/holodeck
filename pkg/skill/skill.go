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

// Package skill provides the holodeck agentic-skill catalog: parsing,
// rendering for each agent target, and installation.
package skill

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

// skillNameRE constrains skill names to a safe, predictable form so the
// name can be used as a path segment without traversal risk. Today the
// catalog is embedded at compile time, but the regex hardens the parser
// against future loaders that read SKILL.md from disk or network.
var skillNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Skill is a single entry in the catalog, parsed from a SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Body        string
}

type frontmatter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// parseSkill splits a SKILL.md file into frontmatter and body and
// returns a Skill. fileName is used only for error messages.
func parseSkill(fileName string, raw []byte) (Skill, error) {
	// Frontmatter format: file starts with "---\n", a YAML block, then
	// "---\n", then the body.
	if !bytes.HasPrefix(raw, []byte("---\n")) {
		return Skill{}, fmt.Errorf("%s: frontmatter delimiter (---) not found at start of file", fileName)
	}
	rest := raw[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return Skill{}, fmt.Errorf("%s: closing frontmatter delimiter (---) not found", fileName)
	}
	fmBytes := rest[:end]
	body := strings.TrimSpace(string(rest[end+len("\n---\n"):]))

	var fm frontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return Skill{}, fmt.Errorf("%s: parsing frontmatter: %w", fileName, err)
	}
	if fm.Name == "" {
		return Skill{}, fmt.Errorf("%s: name is required in frontmatter", fileName)
	}
	if !skillNameRE.MatchString(fm.Name) {
		return Skill{}, fmt.Errorf("%s: invalid skill name %q: must match %s", fileName, fm.Name, skillNameRE)
	}
	if fm.Description == "" {
		return Skill{}, fmt.Errorf("%s: description is required in frontmatter", fileName)
	}
	if body == "" {
		return Skill{}, fmt.Errorf("%s: body is empty", fileName)
	}
	return Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Body:        body,
	}, nil
}
