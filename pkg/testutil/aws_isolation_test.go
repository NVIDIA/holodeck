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
	"net/http"
	"os"
	"os/exec"
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
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9")
	for _, k := range append(testutil.AWSCredentialEnv(), "NO_PROXY", "no_proxy") {
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
		t.Setenv("HTTPS_PROXY", "")
		t.Setenv("NO_PROXY", "localhost")

		want := []string{
			testutil.IsolatedHomeEnv + " is not set: the process was not started by testutil.RunAWSIsolated",
			fmt.Sprintf("HOME=%q is not the isolated HOME %q", home, ""),
			fmt.Sprintf("AWS_CONFIG_FILE=%q is not an empty file inside HOME", "/etc/hosts"),
			fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%q is not an empty file inside HOME", creds),
			`AWS_EC2_METADATA_DISABLED is not "true"`,
			`AWS_ENDPOINT_URL is not "http://127.0.0.1:9"`,
			`HTTPS_PROXY is not "http://127.0.0.1:9"`,
			"NO_PROXY is set",
		}
		assert.Equal(t, append(want, defaultPathProblems()...), testutil.AWSIsolationProblems())
	})

	t.Run("HOME outside os.TempDir is reported", func(t *testing.T) {
		fakeIsolation(t)
		t.Setenv("HOME", "/")
		t.Setenv(testutil.IsolatedHomeEnv, "/")
		assert.Equal(t, []string{fmt.Sprintf("HOME=%q is not inside os.TempDir() %q", "/", os.TempDir())},
			testutil.AWSIsolationProblems())
	})
}

// TestIsolatedProxy proves the proxy backstop: every AWS host, including
// the SSO portal that AWS_ENDPOINT_URL does not cover, goes to the closed
// local port, while loopback (httptest servers, local fixtures) bypasses
// the proxy, as net/http's ProxyFromEnvironment does for localhost and
// loopback addresses.
func TestIsolatedProxy(t *testing.T) {
	tests := []struct {
		url  string
		want string // "" = direct, no proxy
	}{
		{"https://portal.sso.us-east-1.amazonaws.com/federation/credentials", "http://127.0.0.1:9"},
		{"https://oidc.us-east-1.amazonaws.com/token", "http://127.0.0.1:9"},
		{"https://ec2.us-west-2.amazonaws.com/", "http://127.0.0.1:9"},
		{"http://169.254.169.254/latest/api/token", "http://127.0.0.1:9"},
		{"http://127.0.0.1:8080/", ""},
		{"http://localhost:8080/", ""},
		{"http://[::1]:8080/", ""},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			got, err := http.ProxyFromEnvironment(req)
			require.NoError(t, err)
			if tc.want == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got, "AWS traffic must go through the closed proxy")
			assert.Equal(t, tc.want, got.String())
		})
	}
}

// TestRunAWSIsolatedRefusesStaleMarker: a leftover exported
// HOLODECK_TEST_ISOLATED_HOME must not let a test binary run its tests
// un-isolated. The child gets a marker that does not match its HOME; it
// must exit 1 before running any test.
func TestRunAWSIsolatedRefusesStaleMarker(t *testing.T) {
	stale := t.TempDir()
	//nolint:gosec // re-executes this same test binary with a fixed -test.run
	cmd := exec.Command(os.Args[0], "-test.run=^TestAWSIsolation$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), testutil.IsolatedHomeEnv+"="+stale)
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "child output:\n%s", out)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(out),
		"testutil.RunAWSIsolated: refusing to run tests, the process is not isolated:")
	assert.Contains(t, string(out),
		fmt.Sprintf("HOME=%q is not the isolated HOME %q", os.Getenv("HOME"), stale))
	assert.NotContains(t, string(out), "=== RUN", "no test may run")
}

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
