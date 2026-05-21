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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpliceSection_AppendsWhenAbsent(t *testing.T) {
	existing := []byte("# Top of file\n\nSome content.\n")
	section := []byte("<!-- BEGIN holodeck-skill:demo -->\n## demo\n\nbody\n<!-- END holodeck-skill:demo -->\n")
	got := spliceSection(existing, section, "demo")
	if !bytes.Contains(got, []byte("Some content.")) {
		t.Errorf("existing content lost; got:\n%s", got)
	}
	if !bytes.Contains(got, []byte("<!-- BEGIN holodeck-skill:demo -->")) {
		t.Errorf("section marker not appended; got:\n%s", got)
	}
	if bytes.Count(got, []byte("<!-- BEGIN holodeck-skill:demo -->")) != 1 {
		t.Errorf("expected exactly one BEGIN marker; got:\n%s", got)
	}
}

func TestSpliceSection_ReplacesWhenPresent(t *testing.T) {
	existing := []byte("# Top\n\n" +
		"<!-- BEGIN holodeck-skill:demo -->\nOLD CONTENT\n<!-- END holodeck-skill:demo -->\n" +
		"# Bottom\n")
	section := []byte("<!-- BEGIN holodeck-skill:demo -->\nNEW CONTENT\n<!-- END holodeck-skill:demo -->\n")
	got := spliceSection(existing, section, "demo")
	if bytes.Contains(got, []byte("OLD CONTENT")) {
		t.Errorf("old section not replaced; got:\n%s", got)
	}
	if !bytes.Contains(got, []byte("NEW CONTENT")) {
		t.Errorf("new section not present; got:\n%s", got)
	}
	if !bytes.Contains(got, []byte("# Top")) || !bytes.Contains(got, []byte("# Bottom")) {
		t.Errorf("surrounding content damaged; got:\n%s", got)
	}
	if bytes.Count(got, []byte("<!-- BEGIN holodeck-skill:demo -->")) != 1 {
		t.Errorf("expected exactly one BEGIN marker; got:\n%s", got)
	}
}

func TestSpliceSection_OnlyTouchesNamedSkill(t *testing.T) {
	existing := []byte("<!-- BEGIN holodeck-skill:other -->\nOTHER\n<!-- END holodeck-skill:other -->\n")
	section := []byte("<!-- BEGIN holodeck-skill:demo -->\nDEMO\n<!-- END holodeck-skill:demo -->\n")
	got := spliceSection(existing, section, "demo")
	if !bytes.Contains(got, []byte("OTHER")) {
		t.Errorf("other skill's section was modified; got:\n%s", got)
	}
	if !bytes.Contains(got, []byte("DEMO")) {
		t.Errorf("new skill's section not appended; got:\n%s", got)
	}
}

func TestInstall_PerFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out", "SKILL.md")
	content := []byte("hello\n")
	err := writeFileAtomic(dest, content)
	if err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	got, err := os.ReadFile(dest) // #nosec G304 -- dest is constructed from t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestInstall_PerFile_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "a", "b", "c", "SKILL.md")
	if err := writeFileAtomic(dest, []byte("x")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected %s to exist: %v", dest, err)
	}
}

func TestInstall_SingleFile_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(dest, []byte("# Existing\n\nold content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	section := []byte("<!-- BEGIN holodeck-skill:demo -->\nDEMO\n<!-- END holodeck-skill:demo -->\n")

	if err := installSingleFile(dest, section, "demo"); err != nil {
		t.Fatalf("installSingleFile: %v", err)
	}

	got, err := os.ReadFile(dest) // #nosec G304 -- dest is constructed from t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "old content") {
		t.Errorf("existing content lost; got:\n%s", out)
	}
	if !strings.Contains(out, "DEMO") {
		t.Errorf("section not appended; got:\n%s", out)
	}
}

func TestInstall_SingleFile_CreatesIfMissing(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	section := []byte("<!-- BEGIN holodeck-skill:demo -->\nDEMO\n<!-- END holodeck-skill:demo -->\n")

	if err := installSingleFile(dest, section, "demo"); err != nil {
		t.Fatalf("installSingleFile: %v", err)
	}
	got, err := os.ReadFile(dest) // #nosec G304 -- dest is constructed from t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "DEMO") {
		t.Errorf("section not written; got:\n%s", got)
	}
}

func TestInstall_OverwriteCheck_NoFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "missing.md")
	exists, err := destinationExists(dest)
	if err != nil {
		t.Fatalf("destinationExists: %v", err)
	}
	if exists {
		t.Errorf("destinationExists = true, want false for missing file")
	}
}

func TestInstall_OverwriteCheck_FileExists(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "exists.md")
	if err := os.WriteFile(dest, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	exists, err := destinationExists(dest)
	if err != nil {
		t.Fatalf("destinationExists: %v", err)
	}
	if !exists {
		t.Errorf("destinationExists = false, want true for present file")
	}
}
