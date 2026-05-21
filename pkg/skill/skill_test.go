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
	"strings"
	"testing"
)

func TestParseSkill(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		raw         string
		wantName    string
		wantDesc    string
		wantBodyHas string
		wantErr     string
	}{
		{
			name:     "valid skill",
			fileName: "using-holodeck",
			raw: "---\nname: using-holodeck\n" +
				"description: Drive the holodeck CLI\n---\n" +
				"# Body\n\nDo the thing.\n",
			wantName:    "using-holodeck",
			wantDesc:    "Drive the holodeck CLI",
			wantBodyHas: "Do the thing.",
		},
		{
			name:     "missing name",
			fileName: "foo",
			raw:      "---\ndescription: x\n---\nbody\n",
			wantErr:  "name is required",
		},
		{
			name:     "missing description",
			fileName: "foo",
			raw:      "---\nname: foo\n---\nbody\n",
			wantErr:  "description is required",
		},
		{
			name:     "invalid name: path traversal",
			fileName: "evil",
			raw:      "---\nname: ../../etc/passwd\ndescription: x\n---\nbody\n",
			wantErr:  "invalid skill name",
		},
		{
			name:     "invalid name: starts with hyphen",
			fileName: "leading-hyphen",
			raw:      "---\nname: -foo\ndescription: x\n---\nbody\n",
			wantErr:  "invalid skill name",
		},
		{
			name:     "invalid name: uppercase",
			fileName: "uppercase",
			raw:      "---\nname: Foo\ndescription: x\n---\nbody\n",
			wantErr:  "invalid skill name",
		},
		{
			name:     "invalid name: underscore",
			fileName: "underscore",
			raw:      "---\nname: foo_bar\ndescription: x\n---\nbody\n",
			wantErr:  "invalid skill name",
		},
		{
			name:     "invalid name: embedded space",
			fileName: "spaces",
			raw:      "---\nname: foo bar\ndescription: x\n---\nbody\n",
			wantErr:  "invalid skill name",
		},
		{
			name:     "invalid name: exceeds 64 char cap",
			fileName: "toolong",
			// 65 characters: 'a' repeated.
			raw:     "---\nname: " + strings.Repeat("a", 65) + "\ndescription: x\n---\nbody\n",
			wantErr: "invalid skill name",
		},
		{
			name:     "valid name: 64 char upper bound",
			fileName: "boundary",
			// Exactly 64 characters: 'a' repeated. Locks down the {0,63}
			// length cap against a mistaken tightening to {0,62}.
			raw:         "---\nname: " + strings.Repeat("a", 64) + "\ndescription: x\n---\nbody\n",
			wantName:    strings.Repeat("a", 64),
			wantDesc:    "x",
			wantBodyHas: "body",
		},
		{
			name:     "empty body",
			fileName: "foo",
			raw:      "---\nname: foo\ndescription: x\n---\n",
			wantErr:  "body is empty",
		},
		{
			name:     "no frontmatter delimiter",
			fileName: "foo",
			raw:      "name: foo\nbody\n",
			wantErr:  "frontmatter",
		},
		{
			name:     "malformed yaml",
			fileName: "foo",
			raw:      "---\nname: foo\n  bad: indent: here\n---\nbody\n",
			wantErr:  "frontmatter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := parseSkill(tt.fileName, []byte(tt.raw))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", s.Name, tt.wantName)
			}
			if s.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", s.Description, tt.wantDesc)
			}
			if !strings.Contains(s.Body, tt.wantBodyHas) {
				t.Errorf("Body = %q, want substring %q", s.Body, tt.wantBodyHas)
			}
		})
	}
}
