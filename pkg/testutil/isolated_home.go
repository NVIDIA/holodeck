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
	"path/filepath"
	"testing"
)

// IsolatedHomeEnv is set to the isolated HOME in the child process.
const IsolatedHomeEnv = "HOLODECK_TEST_ISOLATED_HOME"

// IsolatedHome returns the isolated HOME of a RunAWSIsolated child, after
// removing its .aws directory; it removes it again when t ends, so each
// (sub)test starts from an empty HOME. Fixtures written here are what the
// SDK's package-init default paths point at, too. It fails t outside a
// child.
func IsolatedHome(t *testing.T) string {
	t.Helper()
	home := os.Getenv(IsolatedHomeEnv)
	if home == "" {
		t.Fatalf("IsolatedHome called outside a RunAWSIsolated child (add the package TestMain)")
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
