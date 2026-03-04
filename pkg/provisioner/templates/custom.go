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
	"strings"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

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

	// Limit read to 10MB to prevent abuse
	content, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("custom template %q: failed to read response: %w", name, err)
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
	// Log header
	fmt.Fprintf(tpl, "\n# === [CUSTOM] Template: %s (phase: %s) ===\n", ct.Name, ct.Phase)
	fmt.Fprintf(tpl, `holodeck_log "INFO" "custom" "[CUSTOM] Running template '%s' (phase: %s)"`+"\n", ct.Name, ct.Phase)

	// Export environment variables with proper shell quoting
	for k, v := range ct.Env {
		fmt.Fprintf(tpl, "export %s=%q\n", k, v)
	}

	// Write the script content with error handling
	if ct.ContinueOnError {
		fmt.Fprintf(tpl, "# continueOnError=true: failures will be logged but not halt provisioning\n")
		fmt.Fprintf(tpl, "set +e\n")
		tpl.Write(ct.Content)
		fmt.Fprintf(tpl, "\n_custom_rc=$?\nset -e\n")
		fmt.Fprintf(tpl, "if [ $_custom_rc -ne 0 ]; then\n")
		fmt.Fprintf(tpl, `  holodeck_log "WARN" "custom" "[CUSTOM] Template '%s' failed (exit code: $_custom_rc) || true"`+"\n", ct.Name)
		fmt.Fprintf(tpl, "fi\n")
	} else {
		tpl.Write(ct.Content)
		fmt.Fprintf(tpl, "\n")
	}

	fmt.Fprintf(tpl, `holodeck_log "INFO" "custom" "[CUSTOM] Template '%s' completed"`+"\n", ct.Name)

	return nil
}
