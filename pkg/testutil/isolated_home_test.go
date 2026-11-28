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

package testutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// TestIsolatedHome: inside the TestMain child, IsolatedHome returns the
// isolated HOME and clears a leftover .aws directory.
func TestIsolatedHome(t *testing.T) {
	home := os.Getenv(testutil.IsolatedHomeEnv)
	require.NotEmpty(t, home)
	first := testutil.IsolatedHome(t)
	stale := filepath.Join(first, ".aws", "config")
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o700))
	require.NoError(t, os.WriteFile(stale, []byte("[default]\n"), 0o600))

	second := testutil.IsolatedHome(t)
	assert.Equal(t, home, first)
	assert.Equal(t, home, second)
	_, err := os.Stat(filepath.Join(second, ".aws"))
	assert.True(t, os.IsNotExist(err), "IsolatedHome must remove a leftover .aws, stat err=%v", err)
}
