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
	"context"
	"crypto/sha1" //nolint:gosec // the SDK names SSO token cache files by SHA-1
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

const (
	testProfile    = "holodeck-sso-test"
	defaultProfile = "holodeck-sso-default"
	testStartURL   = "https://holodeck-sso-test.awsapps.com/start"
	testSession    = "s"
)

// isolateAWSEnv points every SDK input at a fresh temp HOME so no test can
// read the real machine's AWS config, credentials or SSO token cache. It
// returns the temp HOME.
func isolateAWSEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(home, ".aws", "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(home, ".aws", "credentials"))
	for _, k := range []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_ROLE_ARN",
		"AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	// Belt and braces: if the SDK ever tried to reach a service here, it
	// would hit a closed local port instead of AWS.
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:9")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".aws", "sso", "cache"), 0o700))
	return home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// cachePath mirrors ssocreds.StandardCachedTokenFilepath: the cache file is
// the lower-case hex SHA-1 of the key, under ~/.aws/sso/cache.
func cachePath(home, key string) string {
	sum := sha1.Sum([]byte(key)) //nolint:gosec // matches the SDK naming scheme
	return filepath.Join(home, ".aws", "sso", "cache", hex.EncodeToString(sum[:])+".json")
}

func profileHint(p string) string {
	return "AWS SSO session for profile \"" + p + "\" is expired or missing; " +
		"run \"aws sso login --profile " + p + "\" and retry: "
}

const noProfileHint = "AWS SSO session is expired or missing; run \"aws sso login\" and retry: "

func TestSSOHint(t *testing.T) {
	ssoErr := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &ssocreds.InvalidTokenError{}))
	sessionErr := fmt.Errorf("get identity: %w", fmt.Errorf(
		"refresh cached SSO token failed, %w",
		errors.New("cached SSO token is expired, or not present, and cannot be refreshed")))
	boom := errors.New("boom")

	tests := []struct {
		name        string
		profile     string
		defProfile  string
		in          error
		wantMsg     string
		wantSame    bool
		wantTokenAs bool
	}{
		{
			name:     "plain error is returned unchanged",
			profile:  testProfile,
			in:       boom,
			wantMsg:  "boom",
			wantSame: true,
		},
		{
			name:        "typed SSO error with AWS_PROFILE names the profile",
			profile:     testProfile,
			in:          ssoErr,
			wantMsg:     profileHint(testProfile) + "outer: inner: the SSO session has expired or is invalid",
			wantTokenAs: true,
		},
		{
			name:        "typed SSO error without a profile gives the generic hint",
			in:          ssoErr,
			wantMsg:     noProfileHint + "outer: inner: the SSO session has expired or is invalid",
			wantTokenAs: true,
		},
		{
			name:        "typed SSO error falls back to AWS_DEFAULT_PROFILE",
			defProfile:  defaultProfile,
			in:          ssoErr,
			wantMsg:     profileHint(defaultProfile) + "outer: inner: the SSO session has expired or is invalid",
			wantTokenAs: true,
		},
		{
			name:    "AWS_PROFILE wins over AWS_DEFAULT_PROFILE",
			profile: testProfile, defProfile: defaultProfile,
			in:          ssoErr,
			wantMsg:     profileHint(testProfile) + "outer: inner: the SSO session has expired or is invalid",
			wantTokenAs: true,
		},
		{
			name:    "untyped sso-session token error names the profile",
			profile: testProfile,
			in:      sessionErr,
			wantMsg: profileHint(testProfile) + "get identity: refresh cached SSO token failed, " +
				"cached SSO token is expired, or not present, and cannot be refreshed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_PROFILE", tc.profile)
			t.Setenv("AWS_DEFAULT_PROFILE", tc.defProfile)

			got := internalaws.SSOHint(tc.in)
			require.Error(t, got)
			assert.Equal(t, tc.wantMsg, got.Error())
			assert.ErrorIs(t, got, tc.in, "the original error must stay in the chain")
			if tc.wantSame {
				assert.Same(t, tc.in, got)
			}
			var tokenErr *ssocreds.InvalidTokenError
			assert.Equal(t, tc.wantTokenAs, errors.As(got, &tokenErr))
		})
	}

	t.Run("nil stays nil", func(t *testing.T) {
		isolateAWSEnv(t)
		t.Setenv("AWS_PROFILE", testProfile)
		assert.NoError(t, internalaws.SSOHint(nil))
	})
}

const legacyConfig = `[profile holodeck-sso-test]
sso_start_url = ` + testStartURL + `
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = HolodeckTest
region = us-west-2
`

const sessionConfig = `[profile holodeck-sso-test]
sso_session = ` + testSession + `
sso_account_id = 123456789012
sso_role_name = HolodeckTest
region = us-west-2

[sso-session ` + testSession + `]
sso_start_url = ` + testStartURL + `
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`

const expiredLegacyToken = `{"accessToken":"fake-access-token","expiresAt":"2020-01-01T00:00:00Z",` +
	`"region":"us-east-1","startUrl":"` + testStartURL + `"}`

// No refreshToken, clientId or clientSecret, so the SDK cannot refresh it.
const expiredSessionToken = `{"accessToken":"fake-access-token","expiresAt":"2020-01-01T00:00:00Z",` +
	`"region":"us-east-1","startUrl":"` + testStartURL + `"}`

// TestSSOHintRealSDKChain drives a real SSM call through the SDK's default
// credential chain under fake SSO profiles. Credential resolution fails on
// the local token cache before any request is signed, so nothing leaves the
// machine; AWS_ENDPOINT_URL points at a closed local port as a backstop.
func TestSSOHintRealSDKChain(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		cacheKey    string
		token       string
		wantTokenAs bool
	}{
		{
			name:        "legacy profile with an expired cached token",
			config:      legacyConfig,
			cacheKey:    testStartURL,
			token:       expiredLegacyToken,
			wantTokenAs: true,
		},
		{
			name:     "sso-session profile with an expired token and no refresh token",
			config:   sessionConfig,
			cacheKey: testSession,
			token:    expiredSessionToken,
		},
		{
			name:   "sso-session profile with no cached token",
			config: sessionConfig,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := isolateAWSEnv(t)
			writeFile(t, filepath.Join(home, ".aws", "config"), tc.config)
			if tc.token != "" {
				writeFile(t, cachePath(home, tc.cacheKey), tc.token)
			}
			t.Setenv("AWS_PROFILE", testProfile)

			ctx := context.Background()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
			require.NoError(t, err)

			_, err = ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
				Name: aws.String("holodeck-sso-test"),
			})
			require.Error(t, err)
			t.Logf("SDK error: %v", err)

			got := internalaws.SSOHint(err)
			assert.Equal(t, profileHint(testProfile)+err.Error(), got.Error())
			assert.ErrorIs(t, got, err)
			var tokenErr *ssocreds.InvalidTokenError
			assert.Equal(t, tc.wantTokenAs, errors.As(got, &tokenErr))
		})
	}
}
