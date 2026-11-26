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

package aws

import (
	"crypto/sha1" //nolint:gosec // the SDK names SSO token cache files by SHA-1
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/testutil"
)

const ssoTestStartURL = "https://holodeck-sso-test.awsapps.com/start"

// A token that expires far in the future, so it is valid if anything reads it.
const validSSOToken = `{"accessToken":"fake-access-token","expiresAt":"2999-01-01T00:00:00Z",` +
	`"region":"us-east-1","startUrl":"` + ssoTestStartURL + `"}`

// TestNewResolvesSSOProfile guards the default credential chain: with
// AWS_PROFILE naming an SSO profile, New must hand the EC2 client the SDK's
// SSO credentials provider. It never calls Retrieve, so it reads no token
// and makes no network call; New must stay lazy (holodeck list depends on it).
func TestNewResolvesSSOProfile(t *testing.T) {
	if !testutil.RunWithIsolatedHome(t) {
		return
	}
	tests := []struct {
		name        string
		config      string
		cacheKey    string
		credentials string // non-empty: written to the DEFAULT ~/.aws/credentials
	}{
		{
			name: "legacy sso_start_url profile",
			config: `[profile holodeck-sso-test]
sso_start_url = ` + ssoTestStartURL + `
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = HolodeckTest
`,
			cacheKey: ssoTestStartURL,
		},
		{
			name: "sso-session profile",
			config: `[profile holodeck-sso-test]
sso_session = s
sso_account_id = 123456789012
sso_role_name = HolodeckTest

[sso-session s]
sso_start_url = ` + ssoTestStartURL + `
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`,
			cacheKey: "s",
		},
		{
			// Carlos's failure: a stale static key in the default shared
			// credentials file shadowed the SSO login. New must go through
			// internalaws.LoadConfig, which prefers SSO here.
			name: "sso-session profile shadowed by static keys in the default credentials file",
			config: `[profile holodeck-sso-test]
sso_session = s
sso_account_id = 123456789012
sso_role_name = HolodeckTest

[sso-session s]
sso_start_url = ` + ssoTestStartURL + `
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`,
			cacheKey: "s",
			credentials: "[holodeck-sso-test]\naws_access_key_id = AKIAHOLODECKFAKE0001\n" +
				"aws_secret_access_key = holodeck-fake-secret\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The isolated process HOME: the SDK's package-init default
			// paths point here too, so a call site that bypassed
			// internalaws.LoadConfig would read this fixture, never the
			// real ~/.aws.
			home := os.Getenv(testutil.IsolatedHomeEnv)
			require.NotEmpty(t, home)
			require.NoError(t, os.RemoveAll(filepath.Join(home, ".aws")))
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(filepath.Join(home, ".aws"))) })
			t.Setenv("HOME", home)
			t.Setenv("AWS_CONFIG_FILE", filepath.Join(home, ".aws", "config"))
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(home, ".aws", "credentials"))
			for _, k := range []string{
				"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
				"AWS_DEFAULT_PROFILE", "AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
				"AWS_REGION", "AWS_DEFAULT_REGION",
				"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
			} {
				t.Setenv(k, "")
			}
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:9")
			t.Setenv("AWS_PROFILE", "holodeck-sso-test")

			cacheDir := filepath.Join(home, ".aws", "sso", "cache")
			require.NoError(t, os.MkdirAll(cacheDir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(home, ".aws", "config"), []byte(tc.config), 0o600))
			sum := sha1.Sum([]byte(tc.cacheKey)) //nolint:gosec // matches the SDK naming scheme
			require.NoError(t, os.WriteFile(
				filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json"), []byte(validSSOToken), 0o600))
			if tc.credentials != "" {
				require.NoError(t, os.WriteFile(filepath.Join(home, ".aws", "credentials"), []byte(tc.credentials), 0o600))
				// Use the default credentials path under the temp HOME, as a
				// user without AWS_SHARED_CREDENTIALS_FILE would.
				require.NoError(t, os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE"))
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "sso-test"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{Region: "us-west-2"},
				},
			}
			p, err := New(logger.NewLogger(), env, filepath.Join(home, "cache.yaml"))
			require.NoError(t, err)

			client, ok := p.ec2.(*ec2.Client)
			require.True(t, ok, "New must build a real *ec2.Client, got %T", p.ec2)
			creds, ok := client.Options().Credentials.(*awssdk.CredentialsCache)
			require.True(t, ok, "credentials must be a *aws.CredentialsCache, got %T", client.Options().Credentials)

			// ProviderSources is not asserted: the SDK reports the legacy SSO
			// sources for sso-session profiles too (hasLegacySSOConfiguration
			// is true whenever sso_account_id is set).
			assert.True(t, creds.IsCredentialsProvider((*ssocreds.Provider)(nil)),
				"EC2 credentials must come from the SSO provider")
		})
	}
}
