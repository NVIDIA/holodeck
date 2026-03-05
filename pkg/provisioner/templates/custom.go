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
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// envKeyPattern matches valid POSIX shell variable names.
var envKeyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// shellSafeNamePattern matches characters unsafe for interpolation into shell strings.
var shellSafeNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// shellQuote produces a single-quoted shell string, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// sanitizeName strips shell-unsafe characters from a template name for use in shell output.
func sanitizeName(s string) string {
	return shellSafeNamePattern.ReplaceAllString(s, "_")
}

const maxURLResponseBytes = 10 * 1024 * 1024 // 10MB

// sha256Hex computes the hex-encoded SHA256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// LoadCustomTemplate loads script content from the appropriate source.
// baseDir is used to resolve relative file paths (typically the directory
// containing the Holodeck config file).
func LoadCustomTemplate(tpl v1alpha1.CustomTemplate, baseDir string) ([]byte, error) {
	var content []byte
	var err error

	switch {
	case tpl.Inline != "":
		content = []byte(tpl.Inline)
	case tpl.File != "":
		path := tpl.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("custom template %q: failed to read file %q: %w", tpl.Name, path, err)
		}
	case tpl.URL != "":
		content, err = fetchURL(tpl.URL, tpl.Name)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("custom template %q: no source specified", tpl.Name)
	}

	// Verify checksum if provided
	if tpl.Checksum != "" {
		expected := strings.TrimPrefix(tpl.Checksum, "sha256:")
		actual := sha256Hex(content)
		if actual != expected {
			return nil, fmt.Errorf("custom template %q: checksum mismatch: expected %s, got %s", tpl.Name, expected, actual)
		}
	}

	return content, nil
}

// fetchURL downloads content from a URL with a timeout.
func fetchURL(url, name string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("custom template %q: failed to fetch %q: %w", name, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("custom template %q: URL %q returned status %d", name, url, resp.StatusCode)
	}

	// Read one extra byte to detect truncation
	content, err := io.ReadAll(io.LimitReader(resp.Body, maxURLResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("custom template %q: failed to read response: %w", name, err)
	}
	if len(content) > maxURLResponseBytes {
		return nil, fmt.Errorf("custom template %q: URL response exceeds 10MB limit", name)
	}

	return content, nil
}

// CustomTemplateExecutor generates the bash script for a custom template.
type CustomTemplateExecutor struct {
	Name            string
	Phase           v1alpha1.TemplatePhase
	Content         []byte
	Env             map[string]string
	ContinueOnError bool
	Timeout         int
}

// NewCustomTemplateExecutor creates an executor for a loaded custom template.
func NewCustomTemplateExecutor(tpl v1alpha1.CustomTemplate, content []byte) *CustomTemplateExecutor {
	timeout := tpl.Timeout
	if timeout <= 0 {
		timeout = 600
	}
	return &CustomTemplateExecutor{
		Name:            tpl.Name,
		Phase:           tpl.Phase,
		Content:         content,
		Env:             tpl.Env,
		ContinueOnError: tpl.ContinueOnError,
		Timeout:         timeout,
	}
}

// Execute writes the custom template script to the buffer.
func (ct *CustomTemplateExecutor) Execute(tpl *bytes.Buffer, _ v1alpha1.Environment) error {
	// Sanitize name and phase for safe shell interpolation (defense-in-depth)
	safeName := sanitizeName(ct.Name)
	safePhase := sanitizeName(string(ct.Phase))

	// Validate env var keys before generating any output (prevent key injection)
	for k := range ct.Env {
		if !envKeyPattern.MatchString(k) {
			return fmt.Errorf("custom template %q: invalid environment variable name %q", ct.Name, k)
		}
	}

	// Log header
	fmt.Fprintf(tpl, "\n# === [CUSTOM] Template: %s (phase: %s) ===\n", safeName, safePhase)
	fmt.Fprintf(tpl, `holodeck_log \"INFO\" \"custom\" \"[CUSTOM] Running template '%s' (phase: %s)\"`+"\n", safeName, safePhase)

	// Export environment variables with single-quote shell quoting (prevent value injection)
	for k, v := range ct.Env {
		fmt.Fprintf(tpl, "export %s=%s\n", k, shellQuote(v))
	}

	// Write the script content with error handling
	if ct.ContinueOnError {
		fmt.Fprintf(tpl, "# continueOnError=true: failures will be logged but not halt provisioning\n")
		fmt.Fprintf(tpl, "set +e\n")
		tpl.Write(ct.Content)
		fmt.Fprintf(tpl, "\n_custom_rc=$?\nset -e\n")
		fmt.Fprintf(tpl, "if [ $_custom_rc -ne 0 ]; then\n")
		fmt.Fprintf(tpl, `  holodeck_log \"WARN\" \"custom\" \"[CUSTOM] Template '%s' failed (exit code: $_custom_rc)\" || true`+"\n", safeName)
		fmt.Fprintf(tpl, "fi\n")
	} else {
		tpl.Write(ct.Content)
		fmt.Fprintf(tpl, "\n")
	}

	fmt.Fprintf(tpl, `holodeck_log \"INFO\" \"custom\" \"[CUSTOM] Template '%s' completed\"`+"\n", safeName)

	return nil
}
