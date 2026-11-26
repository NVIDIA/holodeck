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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// TestRunWithIsolatedHome checks the property the helper exists for: in the
// child, the AWS SDK's package-init default paths (computed from HOME before
// any test code runs) point into the isolated HOME, never the real one. It
// only reads the path variables; it opens no file.
func TestRunWithIsolatedHome(t *testing.T) {
	if !testutil.RunWithIsolatedHome(t) {
		return
	}
	home := os.Getenv(testutil.IsolatedHomeEnv)
	require.NotEmpty(t, home, "child must see %s", testutil.IsolatedHomeEnv)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"HOME is the isolated dir", os.Getenv("HOME"), home},
		{"SDK default config file is under the isolated HOME",
			config.DefaultSharedConfigFiles[0], filepath.Join(home, ".aws", "config")},
		{"SDK default credentials file is under the isolated HOME",
			config.DefaultSharedCredentialsFiles[0], filepath.Join(home, ".aws", "credentials")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}
