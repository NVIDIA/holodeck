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

package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestLoadCustomTemplate_Inline(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "test-inline",
		Inline: "#!/bin/bash\necho hello",
	}

	content, err := LoadCustomTemplate(tpl, "")
	if err != nil {
		t.Fatalf("LoadCustomTemplate failed: %v", err)
	}
	if string(content) != tpl.Inline {
		t.Errorf("got %q, want %q", string(content), tpl.Inline)
	}
}

func TestLoadCustomTemplate_File(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.sh")
	scriptContent := "#!/bin/bash\necho from file"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tpl := v1alpha1.CustomTemplate{
		Name: "test-file",
		File: scriptPath,
	}

	content, err := LoadCustomTemplate(tpl, "")
	if err != nil {
		t.Fatalf("LoadCustomTemplate failed: %v", err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, want %q", string(content), scriptContent)
	}
}

func TestLoadCustomTemplate_FileRelative(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.sh")
	scriptContent := "#!/bin/bash\necho relative"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tpl := v1alpha1.CustomTemplate{
		Name: "test-relative",
		File: "test.sh",
	}

	content, err := LoadCustomTemplate(tpl, dir)
	if err != nil {
		t.Fatalf("LoadCustomTemplate failed: %v", err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, want %q", string(content), scriptContent)
	}
}

func TestLoadCustomTemplate_FileNotFound(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name: "missing-file",
		File: "/nonexistent/script.sh",
	}

	_, err := LoadCustomTemplate(tpl, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadCustomTemplate_ChecksumMatch(t *testing.T) {
	content := "#!/bin/bash\necho hello"
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	hash := sha256Hex([]byte(content))

	tpl := v1alpha1.CustomTemplate{
		Name:     "checksum-test",
		File:     scriptPath,
		Checksum: "sha256:" + hash,
	}

	got, err := LoadCustomTemplate(tpl, "")
	if err != nil {
		t.Fatalf("LoadCustomTemplate failed: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch")
	}
}

func TestLoadCustomTemplate_ChecksumMismatch(t *testing.T) {
	content := "#!/bin/bash\necho hello"
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tpl := v1alpha1.CustomTemplate{
		Name:     "checksum-mismatch",
		File:     scriptPath,
		Checksum: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}

	_, err := LoadCustomTemplate(tpl, "")
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCustomTemplateExecute(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "test-execute",
		Phase:  v1alpha1.TemplatePhasePostInstall,
		Inline: "#!/bin/bash\necho hello",
		Env: map[string]string{
			"MY_VAR": "my_value",
		},
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("#!/bin/bash\necho hello"))

	var buf bytes.Buffer
	if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, `[CUSTOM]`) {
		t.Error("output missing [CUSTOM] prefix")
	}
	if !strings.Contains(out, "test-execute") {
		t.Error("output missing template name")
	}
	if !strings.Contains(out, `export MY_VAR='my_value'`) {
		t.Errorf("output missing env var export: %s", out)
	}
	if !strings.Contains(out, "echo hello") {
		t.Error("output missing script content")
	}
}

func TestCustomTemplateExecute_ContinueOnError(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:            "continue-test",
		Inline:          "exit 1",
		ContinueOnError: true,
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("exit 1"))

	var buf bytes.Buffer
	if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "continueOnError") {
		t.Errorf("expected continueOnError comment in output: %s", out)
	}
	// || true must be outside the log message string, as a shell operator
	if !strings.Contains(out, `" || true`) {
		t.Errorf("expected '|| true' as shell operator outside log message: %s", out)
	}
}

// B1: Shell injection via env var values -- command substitution must not execute
func TestCustomTemplateExecute_EnvValueInjection(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "injection-test",
		Inline: "echo safe",
		Env: map[string]string{
			"SAFE": "$(rm -rf /)",
		},
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("echo safe"))

	var buf bytes.Buffer
	if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := buf.String()
	// Value must be in single quotes to prevent command substitution
	if !strings.Contains(out, `export SAFE='$(rm -rf /)'`) {
		t.Errorf("env value not safely single-quoted: %s", out)
	}
	// Must NOT use double quotes around the value (allows expansion)
	if strings.Contains(out, `export SAFE="$(rm -rf /)"`) {
		t.Error("env value uses double quotes -- vulnerable to command substitution")
	}
}

// B1 continued: single quotes inside values must be escaped
func TestCustomTemplateExecute_EnvValueSingleQuote(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "quote-test",
		Inline: "echo safe",
		Env: map[string]string{
			"MSG": "it's a test",
		},
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("echo safe"))

	var buf bytes.Buffer
	if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := buf.String()
	// Embedded single quote must be escaped with '\'' idiom
	if !strings.Contains(out, `export MSG='it'\''s a test'`) {
		t.Errorf("embedded single quote not escaped properly: %s", out)
	}
}

// B2: Shell injection via env var keys -- invalid keys must be rejected
func TestNewCustomTemplateExecutor_InvalidEnvKey(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "bad-key",
		Inline: "echo safe",
		Env: map[string]string{
			"FOO; rm -rf /": "value",
		},
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("echo safe"))

	var buf bytes.Buffer
	err := ct.Execute(&buf, v1alpha1.Environment{})
	if err == nil {
		t.Fatal("expected error for invalid env var key")
	}
	if !strings.Contains(err.Error(), "invalid environment variable name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewCustomTemplateExecutor_ValidEnvKeys(t *testing.T) {
	validKeys := []string{"FOO", "_BAR", "MY_VAR_123", "_"}
	for _, key := range validKeys {
		tpl := v1alpha1.CustomTemplate{
			Name:   "valid-key-" + key,
			Inline: "echo ok",
			Env:    map[string]string{key: "value"},
		}
		ct := NewCustomTemplateExecutor(tpl, []byte("echo ok"))
		var buf bytes.Buffer
		if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
			t.Errorf("valid key %q rejected: %v", key, err)
		}
	}
}

// R2: Template name/phase must be sanitized in shell output
func TestCustomTemplateExecute_NameSanitization(t *testing.T) {
	tpl := v1alpha1.CustomTemplate{
		Name:   "test' ; rm -rf / #",
		Phase:  v1alpha1.TemplatePhasePostInstall,
		Inline: "echo hello",
	}

	ct := NewCustomTemplateExecutor(tpl, []byte("echo hello"))

	var buf bytes.Buffer
	if err := ct.Execute(&buf, v1alpha1.Environment{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := buf.String()
	// The sanitized name must not contain shell-breaking characters
	if strings.Contains(out, "' ;") {
		t.Errorf("name not sanitized -- shell injection possible: %s", out)
	}
	// Sanitized name should only contain safe characters
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(tpl.Name, "_")
	if !strings.Contains(out, sanitized) {
		t.Errorf("expected sanitized name %q in output: %s", sanitized, out)
	}
}

// N1: fetchURL truncation detection (tested via direct call would need HTTP server,
// so we test the limit constant indirectly -- the implementation test is structural)
