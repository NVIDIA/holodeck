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

// Package testutil provides shared testing utilities, fixtures, and mocks
// for the holodeck test suite.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory and returns its path along with a
// cleanup function. The cleanup function removes the directory and all
// contents.
func TempDir(t testing.TB) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "holodeck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) } // nolint:errcheck
}

// TempFile creates a temporary file with the given content and returns its
// path along with a cleanup function.
func TempFile(t testing.TB, name, content string) (string, func()) {
	t.Helper()
	dir, cleanup := TempDir(t)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		cleanup()
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path, cleanup
}

// FixturePath returns the path to a test fixture file relative to the
// testdata directory.
func FixturePath(name string) string {
	return filepath.Join("testdata", name)
}

// MustWriteFile writes content to a file, failing the test if an error occurs.
func MustWriteFile(t testing.TB, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// MustReadFile reads a file and returns its content, failing the test if an
// error occurs.
func MustReadFile(t testing.TB, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(content)
}

// SetEnv sets an environment variable and returns a cleanup function that
// restores the original value.
func SetEnv(t testing.TB, key, value string) func() {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	return func() {
		if existed {
			os.Setenv(key, original) // nolint:errcheck
		} else {
			os.Unsetenv(key) // nolint:errcheck
		}
	}
}

// UnsetEnv unsets an environment variable and returns a cleanup function
// that restores the original value.
func UnsetEnv(t testing.TB, key string) func() {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
	return func() {
		if existed {
			os.Setenv(key, original) // nolint:errcheck
		}
	}
}

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string {
	return &s
}

// Int32Ptr returns a pointer to the given int32.
func Int32Ptr(i int32) *int32 {
	return &i
}

// BoolPtr returns a pointer to the given bool.
func BoolPtr(b bool) *bool {
	return &b
}
