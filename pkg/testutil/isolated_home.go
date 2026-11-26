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

package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// IsolatedHomeEnv is set to the isolated HOME in the child process.
const IsolatedHomeEnv = "HOLODECK_TEST_ISOLATED_HOME"

// RunWithIsolatedHome re-runs the calling top-level test in a child test
// process whose HOME is a new, empty directory from process start.
//
// The AWS SDK computes config.DefaultSharedConfigFiles and
// DefaultSharedCredentialsFiles once, at package init, from HOME. A
// t.Setenv("HOME", ...) inside a test comes too late for them, so any code
// path that falls back to those defaults would read the developer's real
// ~/.aws. In the child, those defaults point into the empty isolated HOME.
//
// It returns true in the child, where the caller runs the test body, and
// false in the parent, after failing t if the child failed or did not run
// the test. Call it first, from a top-level test (not a subtest).
func RunWithIsolatedHome(t *testing.T) bool {
	t.Helper()
	if os.Getenv(IsolatedHomeEnv) != "" {
		return true
	}

	home := t.TempDir()
	//nolint:gosec // re-executes this test binary with a fixed, anchored -test.run
	cmd := exec.Command(os.Args[0],
		"-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.v")
	// exec uses the last value of a duplicated key, so these override HOME.
	cmd.Env = append(os.Environ(), "HOME="+home, IsolatedHomeEnv+"="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated-HOME run of %s failed: %v\n%s", t.Name(), err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+t.Name()+" ") {
		t.Fatalf("isolated-HOME run did not run %s:\n%s", t.Name(), out)
	}
	t.Logf("isolated-HOME run (HOME=%s):\n%s", home, out)
	return false
}

// IsolatedHome returns the HOME of a RunWithIsolatedHome child, after
// removing its .aws directory; it removes it again when t ends, so each
// (sub)test starts from an empty HOME. Fixtures written here are what the
// SDK's package-init default paths point at, too. It fails t outside a
// child.
func IsolatedHome(t *testing.T) string {
	t.Helper()
	home := os.Getenv(IsolatedHomeEnv)
	if home == "" {
		t.Fatalf("IsolatedHome called outside a RunWithIsolatedHome child")
	}
	awsDir := filepath.Join(home, ".aws")
	reset := func() {
		//nolint:gosec // G703: awsDir is under the temp HOME this package created
		if err := os.RemoveAll(awsDir); err != nil {
			t.Errorf("reset %s: %v", awsDir, err)
		}
	}
	reset()
	t.Cleanup(reset)
	return home
}
