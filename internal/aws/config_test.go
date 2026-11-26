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

package aws_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// captureStdout returns what fn wrote to os.Stdout. FunLogger.Warning
// prints with fmt.Printf, so this is the only way to observe it (same
// pattern as cmd/cli/list/list_test.go).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func preferSSOWarning(profile string) string {
	return "AWS profile \"" + profile + "\" has SSO configured and static keys in the " +
		"shared credentials file; using SSO, as the aws CLI does. Remove the static keys " +
		"from the credentials file to silence this warning."
}

const staticKeys = "aws_access_key_id = AKIAHOLODECKFAKE0001\n" +
	"aws_secret_access_key = holodeck-fake-secret\n"

const defaultSSOSessionConfig = `[default]
sso_session = s
sso_account_id = 123456789012
sso_role_name = HolodeckTest

[sso-session s]
sso_start_url = ` + testStartURL + `
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`

// TestLoadConfigCredentialPrecedence checks which credentials provider
// LoadConfig hands out. It inspects the provider only and never calls
// Retrieve, so no token is read and nothing leaves the machine.
//
// AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE are UNSET (except where a
// case sets one) so LoadConfig takes the default ~/.aws paths, under the
// temp HOME.
func TestLoadConfigCredentialPrecedence(t *testing.T) {
	if !testutil.RunWithIsolatedHome(t) {
		return
	}
	type want int
	const (
		wantSSO want = iota
		wantSharedStatic
		wantEnvStatic
	)
	tests := []struct {
		name        string
		config      string // ~/.aws/config, "" = no file
		credentials string // ~/.aws/credentials, "" = no file
		noAWSDir    bool
		env         map[string]string
		explicitCF  string // AWS_SHARED_CREDENTIALS_FILE contents, set explicitly
		want        want
		wantSource  aws.CredentialSource
		wantWarning string // "" = stdout must be empty
	}{
		{
			name:        "P1 SSO in config and static keys in credentials file prefers SSO",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			want:        wantSSO,
			wantSource:  aws.CredentialSourceProfileSSO,
			wantWarning: preferSSOWarning("default"),
		},
		{
			name:        "P2 only static keys in credentials file stays static",
			credentials: "[default]\n" + staticKeys,
			want:        wantSharedStatic,
			wantSource:  aws.CredentialSourceProfile,
		},
		{
			name:       "P3 only SSO in config resolves SSO without a warning",
			config:     defaultSSOSessionConfig,
			want:       wantSSO,
			wantSource: aws.CredentialSourceProfileSSO,
		},
		{
			name:        "P4 env static keys win over both files",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			env: map[string]string{ //nolint:gosec // G101: fake credentials, never sent anywhere
				"AWS_ACCESS_KEY_ID":     "AKIAHOLODECKFAKEENV1",
				"AWS_SECRET_ACCESS_KEY": "holodeck-fake-env-secret",
			},
			want:       wantEnvStatic,
			wantSource: aws.CredentialSourceEnvVars,
		},
		{
			name:       "P5 explicit AWS_SHARED_CREDENTIALS_FILE is respected",
			config:     defaultSSOSessionConfig,
			explicitCF: "[default]\n" + staticKeys,
			want:       wantSharedStatic,
			wantSource: aws.CredentialSourceProfile,
		},
		{
			name: "P6 AWS_PROFILE=dev with SSO and static keys prefers SSO for dev",
			config: `[profile dev]
sso_start_url = ` + testStartURL + `
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = HolodeckTest
`,
			credentials: "[default]\n" + staticKeys + "\n[dev]\n" + staticKeys,
			env:         map[string]string{"AWS_PROFILE": "dev"},
			want:        wantSSO,
			wantSource:  aws.CredentialSourceSSOLegacy,
			wantWarning: preferSSOWarning("dev"),
		},
		{
			name:     "P7 no ~/.aws at all with env static keys",
			noAWSDir: true,
			env: map[string]string{ //nolint:gosec // G101: fake credentials, never sent anywhere
				"AWS_ACCESS_KEY_ID":     "AKIAHOLODECKFAKEENV1",
				"AWS_SECRET_ACCESS_KEY": "holodeck-fake-env-secret",
			},
			want:       wantEnvStatic,
			wantSource: aws.CredentialSourceEnvVars,
		},
		{
			// The keys live in the config file, so dropping the credentials
			// file cannot make SSO win; there must be no false warning.
			name:       "P8 static keys in the config file next to SSO stay static without a warning",
			config:     strings.Replace(defaultSSOSessionConfig, "[default]\n", "[default]\n"+staticKeys, 1),
			want:       wantSharedStatic,
			wantSource: aws.CredentialSourceProfile,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := isolateAWSEnv(t)
			// isolateAWSEnv pins both file env vars; unset them so the
			// default-path logic is what runs. t.Setenv restores them.
			require.NoError(t, os.Unsetenv("AWS_CONFIG_FILE"))
			require.NoError(t, os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE"))
			if tc.noAWSDir {
				require.NoError(t, os.RemoveAll(filepath.Join(home, ".aws")))
			}
			if tc.config != "" {
				writeFile(t, filepath.Join(home, ".aws", "config"), tc.config)
			}
			if tc.credentials != "" {
				writeFile(t, filepath.Join(home, ".aws", "credentials"), tc.credentials)
			}
			if tc.explicitCF != "" {
				path := filepath.Join(home, "explicit-credentials")
				writeFile(t, path, tc.explicitCF)
				t.Setenv("AWS_SHARED_CREDENTIALS_FILE", path)
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			var cfg aws.Config
			var err error
			out := captureStdout(t, func() {
				cfg, err = internalaws.LoadConfig(context.Background(), logger.NewLogger(), "us-west-2")
			})
			require.NoError(t, err)
			assert.Equal(t, "us-west-2", cfg.Region)

			creds, ok := cfg.Credentials.(*aws.CredentialsCache)
			require.True(t, ok, "credentials must be a *aws.CredentialsCache, got %T", cfg.Credentials)
			sources := creds.ProviderSources()
			isSSO := creds.IsCredentialsProvider((*ssocreds.Provider)(nil))

			switch tc.want {
			case wantSSO:
				assert.True(t, isSSO, "want the SSO provider, sources=%v", sources)
				assert.NotContains(t, sources, aws.CredentialSourceProfile)
			case wantSharedStatic, wantEnvStatic:
				assert.False(t, isSSO, "want static credentials, sources=%v", sources)
			}
			assert.Contains(t, sources, tc.wantSource)

			if tc.wantWarning == "" {
				assert.Empty(t, out, "no output expected")
			} else {
				assert.Equal(t, 1, strings.Count(out, tc.wantWarning), "warning exactly once; stdout=%q", out)
				assert.Equal(t, 1, strings.Count(out, "\n"), "exactly one line; stdout=%q", out)
			}
		})
	}
}
