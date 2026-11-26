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

package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/testutil"
)

// TestNewPrefersSSOOverSharedFileKeys guards the cleanup wiring: with an SSO
// profile in the config file and static keys for it in the default shared
// credentials file, New must load through internalaws.LoadConfig and hand
// the EC2 client the SSO provider. Retrieve is never called.
func TestNewPrefersSSOOverSharedFileKeys(t *testing.T) {
	if !testutil.RunWithIsolatedHome(t) {
		return
	}
	tests := []struct {
		name    string
		wantSSO bool
		creds   string
	}{
		{
			name:    "static keys in the default credentials file do not shadow SSO",
			wantSSO: true,
			creds: "[holodeck-sso-test]\naws_access_key_id = AKIAHOLODECKFAKE0001\n" +
				"aws_secret_access_key = holodeck-fake-secret\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The isolated process HOME: the SDK's package-init default
			// paths point here too, so a call site that bypassed
			// internalaws.LoadConfig would read this fixture, never the
			// real ~/.aws.
			home := testutil.IsolatedHome(t)
			t.Setenv("HOME", home)
			for _, k := range []string{
				"AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE",
				"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
				"AWS_DEFAULT_PROFILE", "AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
				"AWS_REGION", "AWS_DEFAULT_REGION",
				"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
			} {
				t.Setenv(k, "")
				require.NoError(t, os.Unsetenv(k))
			}
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:9")
			t.Setenv("AWS_PROFILE", "holodeck-sso-test")

			awsDir := filepath.Join(home, ".aws")
			require.NoError(t, os.MkdirAll(awsDir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(awsDir, "config"), []byte(`[profile holodeck-sso-test]
sso_session = s
sso_account_id = 123456789012
sso_role_name = HolodeckTest

[sso-session s]
sso_start_url = https://holodeck-sso-test.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(awsDir, "credentials"), []byte(tc.creds), 0o600))

			c, err := New(logger.NewLogger(), "us-west-2")
			require.NoError(t, err)

			client, ok := c.ec2.(*ec2.Client)
			require.True(t, ok, "New must build a real *ec2.Client, got %T", c.ec2)
			creds, ok := client.Options().Credentials.(*awssdk.CredentialsCache)
			require.True(t, ok, "credentials must be a *aws.CredentialsCache, got %T", client.Options().Credentials)
			assert.Equal(t, tc.wantSSO, creds.IsCredentialsProvider((*ssocreds.Provider)(nil)),
				"sources=%v", creds.ProviderSources())
		})
	}
}
