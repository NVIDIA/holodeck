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
	"encoding/json"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/internal/logger"
	pkgskill "github.com/NVIDIA/holodeck/pkg/skill"
)

func TestListAction_TableContainsCatalogEntries(t *testing.T) {
	skills, err := pkgskill.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("Catalog empty; cannot test list output")
	}

	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), outputFormat: "table", out: &buf}
	if err := c.runList(nil); err != nil {
		t.Fatalf("runList: %v", err)
	}
	out := buf.String()
	for _, s := range skills {
		if !strings.Contains(out, s.Name) {
			t.Errorf("list output missing skill %q\nfull output:\n%s", s.Name, out)
		}
	}
}

func TestListAction_JSONShape(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), outputFormat: "json", out: &buf}
	if err := c.runList(nil); err != nil {
		t.Fatalf("runList: %v", err)
	}
	var parsed struct {
		Skills []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if len(parsed.Skills) == 0 {
		t.Errorf("expected at least one skill in JSON output; got %s", buf.String())
	}
}

func TestListAction_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	c := &command{log: logger.NewLogger(), outputFormat: "xml", out: &buf}
	err := c.runList(nil)
	if err == nil {
		t.Errorf("expected error for invalid format, got nil")
	}
}
