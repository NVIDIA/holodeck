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

package os_test

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // the SDK names SSO token cache files by SHA-1
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v3"

	oscmd "github.com/NVIDIA/holodeck/cmd/cli/os"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/testutil"
)

const amiStartURL = "https://holodeck-sso-test.awsapps.com/start"

// TestAMIUsesLoadConfig guards the "holodeck os ami" wiring. The fixture is
// Carlos's layout: an SSO profile in the config file, stale static keys for
// it in the default credentials file, and an expired cached SSO token.
// Through internalaws.LoadConfig the command prefers SSO and fails on the
// expired token with the typed SSO error. Raw config.LoadDefaultConfig
// would take the static keys and fail on the closed local endpoint instead.
// No request reaches AWS either way (package TestMain).
func TestAMIUsesLoadConfig(t *testing.T) {
	home := testutil.IsolatedHome(t)
	t.Setenv("HOME", home)
	for _, k := range []string{"AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE"} {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	cacheDir := filepath.Join(home, ".aws", "sso", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".aws", "config"), []byte(`[default]
sso_start_url = `+amiStartURL+`
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = HolodeckTest
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".aws", "credentials"), []byte(
		"[default]\naws_access_key_id = AKIAHOLODECKFAKE0001\naws_secret_access_key = holodeck-fake-secret\n"), 0o600))
	sum := sha1.Sum([]byte(amiStartURL)) //nolint:gosec // matches the SDK naming scheme
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json"), []byte(
		`{"accessToken":"fake-access-token","expiresAt":"2020-01-01T00:00:00Z","region":"us-east-1","startUrl":"`+
			amiStartURL+`"}`), 0o600))

	log := logger.NewLogger()
	var stderr bytes.Buffer
	log.Out = &stderr
	app := &cli.Command{Commands: []*cli.Command{oscmd.NewCommand(log)}}
	err := app.Run(context.Background(), []string{"holodeck", "os", "ami", "ubuntu-22.04", "--region", "us-west-2"})

	require.Error(t, err)
	var tokenErr *ssocreds.InvalidTokenError
	assert.True(t, errors.As(err, &tokenErr),
		"os ami must load through internalaws.LoadConfig, which prefers the SSO login; err=%v", err)
}
