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
	"testing"
)

func TestCatalogLoadsEmbeddedSkills(t *testing.T) {
	skills, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog returned error: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("Catalog returned empty list; expected at least one embedded skill")
	}
	var found bool
	for _, s := range skills {
		if s.Name == "using-holodeck" {
			found = true
			if s.Description == "" {
				t.Errorf("using-holodeck has empty Description")
			}
			if s.Body == "" {
				t.Errorf("using-holodeck has empty Body")
			}
			break
		}
	}
	if !found {
		t.Errorf("did not find skill named 'using-holodeck' in catalog; got: %v", skillNames(skills))
	}
}

func TestCatalogIsSorted(t *testing.T) {
	skills, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog returned error: %v", err)
	}
	for i := 1; i < len(skills); i++ {
		if skills[i-1].Name > skills[i].Name {
			t.Errorf("Catalog not sorted: %q came before %q", skills[i-1].Name, skills[i].Name)
		}
	}
}

func skillNames(skills []Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name
	}
	return out
}
