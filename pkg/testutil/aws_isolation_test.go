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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// fakeIsolation sets up, inside this (NOT re-executed) test process, every
// isolation property that environment variables can provide. What it cannot
// provide is the SDK's package-init default paths, which were computed from
// the real HOME before this test started.
func fakeIsolation(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "isolated-aws")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	for _, f := range []string{"config", "credentials"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), nil, 0o600))
	}
	t.Setenv("HOME", home)
	t.Setenv(testutil.IsolatedHomeEnv, home)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:9")
	for _, k := range testutil.AWSCredentialEnv() {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	return home
}

func defaultPathProblems() []string {
	return []string{
		fmt.Sprintf("SDK default config file %q (computed at package init) is outside the isolated HOME",
			config.DefaultSharedConfigFiles[0]),
		fmt.Sprintf("SDK default credentials file %q (computed at package init) is outside the isolated HOME",
			config.DefaultSharedCredentialsFiles[0]),
	}
}

func TestAWSIsolationProblems(t *testing.T) {
	t.Run("credential variables cover the brief list and the SDK alternates", func(t *testing.T) {
		assert.Equal(t, []string{
			"AWS_PROFILE", "AWS_DEFAULT_PROFILE",
			"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
			"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
			// The SDK also reads these (config/env_config.go:23-33).
			"AWS_ACCESS_KEY", "AWS_SECRET_KEY",
			"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
			"AWS_CONTAINER_AUTHORIZATION_TOKEN",
		}, testutil.AWSCredentialEnv())
	})

	t.Run("env-only isolation still reports the package-init SDK paths", func(t *testing.T) {
		fakeIsolation(t)
		assert.Equal(t, defaultPathProblems(), testutil.AWSIsolationProblems())
	})

	t.Run("each credential variable is reported when set", func(t *testing.T) {
		for _, k := range testutil.AWSCredentialEnv() {
			t.Run(k, func(t *testing.T) {
				fakeIsolation(t)
				t.Setenv(k, "x")
				assert.Equal(t, append([]string{k + " is set"}, defaultPathProblems()...),
					testutil.AWSIsolationProblems())
			})
		}
	})

	t.Run("missing marker, foreign HOME, non-empty files, IMDS and endpoint are reported", func(t *testing.T) {
		home := fakeIsolation(t)
		creds := filepath.Join(home, "isolated-aws", "credentials")
		require.NoError(t, os.WriteFile(creds, []byte("[default]\n"), 0o600))
		t.Setenv(testutil.IsolatedHomeEnv, "")
		t.Setenv("AWS_CONFIG_FILE", "/etc/hosts")
		t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
		t.Setenv("AWS_ENDPOINT_URL", "")

		want := []string{
			testutil.IsolatedHomeEnv + " is not set: the process was not started by testutil.RunAWSIsolated",
			fmt.Sprintf("HOME=%q is not the isolated HOME %q", home, ""),
			fmt.Sprintf("AWS_CONFIG_FILE=%q is not an empty file inside HOME", "/etc/hosts"),
			fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%q is not an empty file inside HOME", creds),
			`AWS_EC2_METADATA_DISABLED is not "true"`,
			`AWS_ENDPOINT_URL is not "http://127.0.0.1:9"`,
		}
		assert.Equal(t, append(want, defaultPathProblems()...), testutil.AWSIsolationProblems())
	})

	t.Run("HOME outside os.TempDir is reported", func(t *testing.T) {
		fakeIsolation(t)
		t.Setenv("HOME", "/")
		t.Setenv(testutil.IsolatedHomeEnv, "/")
		problems := testutil.AWSIsolationProblems()
		assert.Contains(t, problems, fmt.Sprintf("HOME=%q is not inside os.TempDir() %q", "/", os.TempDir()))
	})
}
