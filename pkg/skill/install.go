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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
)

// InstallOptions controls how a skill is written to disk.
type InstallOptions struct {
	Global bool
	Force  bool
	DryRun bool
	Stdin  io.Reader   // for overwrite prompt; defaults to os.Stdin
	Stdout io.Writer   // for prompts and dry-run output; defaults to os.Stdout
	Stderr io.Writer   // for warnings; defaults to os.Stderr
	IsTTY  func() bool // override TTY check; defaults to isatty on Stdin
}

// Install renders the skill via r and writes it to the renderer's
// install path. Returns the path written to.
//
// For per-file renderers (Claude, Cursor), the rendered bytes become
// the entire file. If the destination exists and Force is false:
//   - TTY → prompt y/N
//   - Non-TTY → error out with a message mentioning --force
//
// For single-file renderers (Codex, Gemini), the rendered bytes are
// a section block, and the existing file's other content is preserved.
// Re-installing the same skill is idempotent.
func Install(r Renderer, s Skill, opts InstallOptions) (string, error) {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.IsTTY == nil {
		opts.IsTTY = func() bool {
			f, ok := opts.Stdin.(*os.File)
			return ok && isatty.IsTerminal(f.Fd())
		}
	}

	dest, err := r.InstallPath(s, opts.Global)
	if err != nil {
		return "", err
	}
	rendered, err := r.Render(s)
	if err != nil {
		return "", err
	}

	if opts.DryRun {
		if _, err := fmt.Fprintf(opts.Stdout, "would write %s (%d bytes)\n", dest, len(rendered)); err != nil {
			return "", err
		}
		return dest, nil
	}

	if r.SingleFile() {
		if err := installSingleFile(dest, rendered, s.Name); err != nil {
			return "", err
		}
		return dest, nil
	}

	// Per-file flow with overwrite prompt.
	exists, err := destinationExists(dest)
	if err != nil {
		return "", err
	}
	if exists && !opts.Force {
		if !opts.IsTTY() {
			return "", fmt.Errorf("%s exists; pass --force to overwrite", dest)
		}
		ok, err := confirmOverwrite(opts.Stdin, opts.Stdout, dest)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("aborted; %s left unchanged", dest)
		}
	}
	if err := writeFileAtomic(dest, rendered); err != nil {
		return "", err
	}
	return dest, nil
}

// destinationExists reports whether dest exists. os.IsNotExist returns
// (false, nil); any other stat error surfaces so the caller never
// silently clobbers a file it couldn't inspect.
func destinationExists(dest string) (bool, error) {
	_, err := os.Stat(dest)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("inspecting %s: %w", dest, err)
	}
}

// writeFileAtomic creates parent dirs as needed and writes content
// to dest via a sibling tempfile + rename.
func writeFileAtomic(dest string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(dest), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".tmp.*")
	if err != nil {
		return fmt.Errorf("opening tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing tempfile %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("renaming %s -> %s: %w", tmpPath, dest, err)
	}
	cleanup = false
	return nil
}

// installSingleFile splices section into the existing file at dest,
// creating the file if it does not exist. The write is atomic.
func installSingleFile(dest string, section []byte, skillName string) error {
	var existing []byte
	// #nosec G304 -- dest is computed from Renderer.InstallPath, bounded to known config-dir paths
	raw, err := os.ReadFile(dest)
	switch {
	case err == nil:
		existing = raw
	case errors.Is(err, os.ErrNotExist):
		existing = nil
	default:
		return fmt.Errorf("reading %s: %w", dest, err)
	}
	merged := spliceSection(existing, section, skillName)
	return writeFileAtomic(dest, merged)
}

// spliceSection inserts section into existing. If a section block
// for skillName is already present (BEGIN ... END markers), it is
// replaced; otherwise section is appended (with a leading blank line
// if existing has content).
func spliceSection(existing, section []byte, skillName string) []byte {
	begin := fmt.Appendf(nil, "<!-- BEGIN holodeck-skill:%s -->", skillName)
	end := fmt.Appendf(nil, "<!-- END holodeck-skill:%s -->", skillName)

	bIdx := bytes.Index(existing, begin)
	if bIdx < 0 {
		// Append.
		var buf bytes.Buffer
		buf.Write(existing)
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			buf.WriteByte('\n')
		}
		if len(existing) > 0 {
			buf.WriteByte('\n')
		}
		buf.Write(section)
		return buf.Bytes()
	}
	// Replace from BEGIN up to and including the trailing newline of END.
	eIdx := bytes.Index(existing[bIdx:], end)
	if eIdx < 0 {
		// Malformed existing file (BEGIN without END); append the new
		// section instead of risking destructive replace.
		var buf bytes.Buffer
		buf.Write(existing)
		if existing[len(existing)-1] != '\n' {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
		buf.Write(section)
		return buf.Bytes()
	}
	endLineEnd := bIdx + eIdx + len(end)
	// Consume the newline that follows END, if any.
	if endLineEnd < len(existing) && existing[endLineEnd] == '\n' {
		endLineEnd++
	}
	var buf bytes.Buffer
	buf.Write(existing[:bIdx])
	buf.Write(section)
	buf.Write(existing[endLineEnd:])
	return buf.Bytes()
}

// confirmOverwrite reads a y/N response from in.
func confirmOverwrite(in io.Reader, out io.Writer, dest string) (bool, error) {
	if _, err := fmt.Fprintf(out, "overwrite %s? [y/N] ", dest); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("reading overwrite confirmation: %w", err)
		}
		return false, nil
	}
	ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return ans == "y" || ans == "yes", nil
}
