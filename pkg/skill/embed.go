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
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed all:data/skills
var catalogFS embed.FS

// Catalog returns every skill embedded in the binary, sorted by Name.
func Catalog() ([]Skill, error) {
	var skills []Skill
	err := fs.WalkDir(catalogFS, "data/skills", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(p, "/SKILL.md") {
			return nil
		}
		raw, err := catalogFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		// Skill name comes from the parent directory name (one level up
		// from SKILL.md): data/skills/<name>/SKILL.md.
		fileName := path.Base(path.Dir(p))
		s, err := parseSkill(fileName, raw)
		if err != nil {
			return err
		}
		skills = append(skills, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}
