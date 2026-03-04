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
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
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
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
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
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
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
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
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
	if !strings.Contains(out, `export MY_VAR="my_value"`) {
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
	if !strings.Contains(out, "|| true") || !strings.Contains(out, "continueOnError") {
		t.Errorf("expected continueOnError handling in output: %s", out)
	}
}
