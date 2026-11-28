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

const roleARN = "role_arn = arn:aws:iam::123456789012:role/Target\n"

// withDefault inserts extra lines at the top of the [default] section of an
// SSO config.
func withDefault(cfg, lines string) string {
	return strings.Replace(cfg, "[default]\n", "[default]\n"+lines, 1)
}

// loadResult is one LoadConfig call: its results, and what it wrote to
// stdout and to the logger's stderr writer.
type loadResult struct {
	cfg    aws.Config
	err    error
	stdout string
	stderr string
}

// loadConfig runs LoadConfig with a logger whose Out is a buffer, re-arming
// the once-per-process warning first.
func loadConfig(t *testing.T) loadResult {
	t.Helper()
	internalaws.ResetPreferSSOWarning()
	log := logger.NewLogger()
	var stderr bytes.Buffer
	log.Out = &stderr
	var cfg aws.Config
	var err error
	stdout := captureStdout(t, func() {
		cfg, err = internalaws.LoadConfig(context.Background(), log, "us-west-2")
	})
	return loadResult{cfg: cfg, err: err, stdout: stdout, stderr: stderr.String()}
}

// TestLoadConfigCredentialPrecedence pins which credentials provider
// LoadConfig hands out, as EXACT SDK credential sources, and how many
// warnings it prints. It inspects the provider only and never calls
// Retrieve, so no token is read and nothing leaves the machine.
//
// The rule matches the aws CLI (botocore) order, as chosen for holodeck:
// SSO wins over static keys from the shared credentials file, including
// when the profile also has credential_process (A2) or a container URI is
// set (A7). It must NOT flip when the profile assumes a role (A1, A1b,
// A1c, A4, A5), when web identity comes from the environment (A6), or when
// the profile does not exist (A9).
//
// AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE are UNSET (except where a
// case sets one) so LoadConfig takes the default ~/.aws paths, under the
// isolated HOME.
func TestLoadConfigCredentialPrecedence(t *testing.T) {
	const (
		envVars        = aws.CredentialSourceEnvVars
		envWebID       = aws.CredentialSourceEnvVarsSTSWebIDToken
		assumeRole     = aws.CredentialSourceSTSAssumeRole
		profile        = aws.CredentialSourceProfile
		sourceProfile  = aws.CredentialSourceProfileSourceProfile
		profileSSO     = aws.CredentialSourceProfileSSO
		profileSSOLegy = aws.CredentialSourceProfileSSOLegacy
		ssoLegacy      = aws.CredentialSourceSSOLegacy
	)
	// sso-session profiles also carry the legacy labels, because
	// hasLegacySSOConfiguration is true whenever sso_account_id is set.
	ssoSession := []aws.CredentialSource{profileSSOLegy, ssoLegacy, profileSSO}
	fakeEnvKeys := map[string]string{ //nolint:gosec // G101: fake credentials, never sent anywhere
		"AWS_ACCESS_KEY_ID":     "AKIAHOLODECKFAKEENV1",
		"AWS_SECRET_ACCESS_KEY": "holodeck-fake-env-secret",
	}
	tests := []struct {
		name        string
		config      string // ~/.aws/config, "" = no file
		credentials string // ~/.aws/credentials, "" = no file
		noAWSDir    bool
		env         map[string]string
		explicitCF  string // AWS_SHARED_CREDENTIALS_FILE contents, set explicitly
		wantSSO     bool
		wantSources []aws.CredentialSource
		wantErr     string
		warnProfile string // "" = no warning
	}{
		{
			name:        "P1 SSO in config and static keys in credentials file prefers SSO",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			wantSSO:     true, wantSources: ssoSession, warnProfile: "default",
		},
		{
			name:        "P2 only static keys in credentials file stays static",
			credentials: "[default]\n" + staticKeys,
			wantSources: []aws.CredentialSource{profile},
		},
		{
			name:    "P3 only SSO in config resolves SSO without a warning",
			config:  defaultSSOSessionConfig,
			wantSSO: true, wantSources: ssoSession,
		},
		{
			name:        "P4 env static keys win over both files",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			env:         fakeEnvKeys,
			wantSources: []aws.CredentialSource{envVars},
		},
		{
			name:        "P5 explicit AWS_SHARED_CREDENTIALS_FILE is respected",
			config:      defaultSSOSessionConfig,
			explicitCF:  "[default]\n" + staticKeys,
			wantSources: []aws.CredentialSource{profile},
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
			wantSSO:     true,
			wantSources: []aws.CredentialSource{profileSSOLegy, ssoLegacy},
			warnProfile: "dev",
		},
		{
			name:        "P7 no ~/.aws at all with env static keys",
			noAWSDir:    true,
			env:         fakeEnvKeys,
			wantSources: []aws.CredentialSource{envVars},
		},
		{
			// The keys live in the config file, so dropping the credentials
			// file cannot make SSO win; there must be no false warning.
			name:        "P8 static keys in the config file next to SSO stay static without a warning",
			config:      withDefault(defaultSSOSessionConfig, staticKeys),
			wantSources: []aws.CredentialSource{profile},
		},
		{
			name:        "A1 role_arn and source_profile in the credentials file do not flip",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys + roleARN + "source_profile = base\n\n[base]\n" + staticKeys,
			wantSources: []aws.CredentialSource{sourceProfile, profile, assumeRole},
		},
		{
			// Dropping the credentials file would also drop this role_arn:
			// holodeck would sign as the bare SSO identity instead.
			name:        "A1b role_arn without source_profile in the credentials file does not flip",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys + roleARN,
			wantSources: []aws.CredentialSource{profile, assumeRole},
		},
		{
			name:        "A1c role_arn in the config file does not flip",
			config:      withDefault(defaultSSOSessionConfig, roleARN),
			credentials: "[default]\n" + staticKeys,
			wantSources: []aws.CredentialSource{profile, assumeRole},
		},
		{
			name:        "A2 credential_process with SSO and credentials-file keys flips, as the aws CLI does",
			config:      withDefault(defaultSSOSessionConfig, "credential_process = /bin/false\n"),
			credentials: "[default]\n" + staticKeys,
			wantSSO:     true, wantSources: ssoSession, warnProfile: "default",
		},
		{
			name: "A4 web_identity_token_file with role_arn in the config does not flip",
			config: withDefault(defaultSSOSessionConfig,
				roleARN+"web_identity_token_file = /nonexistent-token\n"),
			credentials: "[default]\n" + staticKeys,
			wantSources: []aws.CredentialSource{profile, assumeRole},
		},
		{
			name: "A5 credential_source with role_arn does not flip",
			config: withDefault(defaultSSOSessionConfig,
				roleARN+"credential_source = Ec2InstanceMetadata\n"),
			credentials: "[default]\n" + staticKeys,
			wantSources: []aws.CredentialSource{profile, assumeRole},
		},
		{
			name:        "A6 web identity in the environment does not flip",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			env: map[string]string{
				"AWS_WEB_IDENTITY_TOKEN_FILE": "/nonexistent-token",
				"AWS_ROLE_ARN":                "arn:aws:iam::123456789012:role/Target",
			},
			wantSources: []aws.CredentialSource{envWebID},
		},
		{
			name:        "A7 a container credentials URI with SSO and credentials-file keys flips, as the aws CLI does",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			env: map[string]string{ //nolint:gosec // G101: a closed local URL, not a credential
				"AWS_CONTAINER_CREDENTIALS_FULL_URI": "http://127.0.0.1:9/creds",
			},
			wantSSO: true, wantSources: ssoSession, warnProfile: "default",
		},
		{
			name:        "A9 a missing profile fails like the SDK, without a warning",
			config:      defaultSSOSessionConfig,
			credentials: "[default]\n" + staticKeys,
			env:         map[string]string{"AWS_PROFILE": "nope"},
			wantErr:     "failed to get shared config profile, nope",
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

			r := loadConfig(t)
			cfg, err, stdout, stderr := r.cfg, r.err, r.stdout, r.stderr
			assert.Empty(t, stdout, "nothing may go to stdout")
			if tc.warnProfile == "" {
				assert.NotContains(t, stderr, "has SSO configured", "no warning expected")
			} else {
				assert.Equal(t, 1, strings.Count(stderr, preferSSOWarning(tc.warnProfile)),
					"warning exactly once on stderr; stderr=%q", stderr)
				assert.Equal(t, 1, strings.Count(stderr, "\n"), "exactly one line; stderr=%q", stderr)
			}
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "us-west-2", cfg.Region)

			creds, ok := cfg.Credentials.(*aws.CredentialsCache)
			require.True(t, ok, "credentials must be a *aws.CredentialsCache, got %T", cfg.Credentials)
			assert.Equal(t, tc.wantSSO, creds.IsCredentialsProvider((*ssocreds.Provider)(nil)),
				"SSO provider; sources=%v", creds.ProviderSources())
			assert.Equal(t, tc.wantSources, creds.ProviderSources())
		})
	}
}

// TestLoadConfigWarnsOncePerProcess covers holodeck list, which loads a
// config for every cached AWS instance: the warning must appear once per
// process, on stderr, and never on stdout (list -o json must stay JSON).
func TestLoadConfigWarnsOncePerProcess(t *testing.T) {
	home := isolateAWSEnv(t)
	require.NoError(t, os.Unsetenv("AWS_CONFIG_FILE"))
	require.NoError(t, os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE"))
	writeFile(t, filepath.Join(home, ".aws", "config"), defaultSSOSessionConfig)
	writeFile(t, filepath.Join(home, ".aws", "credentials"), "[default]\n"+staticKeys)

	internalaws.ResetPreferSSOWarning()
	log := logger.NewLogger()
	var stderr bytes.Buffer
	log.Out = &stderr
	stdout := captureStdout(t, func() {
		for i := 0; i < 3; i++ {
			cfg, err := internalaws.LoadConfig(context.Background(), log, "us-west-2")
			require.NoError(t, err)
			assert.True(t, aws.IsCredentialsProvider(cfg.Credentials, (*ssocreds.Provider)(nil)),
				"call %d must still prefer SSO", i+1)
		}
	})
	assert.Empty(t, stdout, "nothing may go to stdout")
	assert.Equal(t, 1, strings.Count(stderr.String(), preferSSOWarning("default")), "stderr=%q", stderr.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"), "stderr=%q", stderr.String())
}
