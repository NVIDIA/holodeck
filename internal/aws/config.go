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
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"

	"github.com/NVIDIA/holodeck/internal/logger"
)

// preferSSOWarnOnce limits the prefer-SSO warning to one per process:
// holodeck list loads a config for every cached AWS instance.
var preferSSOWarnOnce sync.Once

const preferSSOWarning = "AWS profile %q has SSO configured and static keys in the shared " +
	"credentials file; using SSO, as the aws CLI does. Remove the static keys from the " +
	"credentials file to silence this warning."

// LoadConfig loads the AWS SDK configuration for region through the SDK
// default chain, with one change: when the active profile has SSO configured
// AND static keys in the default shared credentials file, it uses SSO, as the
// aws CLI does, and logs a warning. The Go SDK otherwise lets those (often
// stale) keys shadow the SSO login.
//
// The SSO preference applies only when nothing explicit overrides it: no
// static keys in the environment, no AWS_SHARED_CREDENTIALS_FILE, and no
// role_arn on the profile (the reload reads no credentials file, so it would
// drop a role that lives there). If the profile cannot be read, or the
// reload does not actually resolve to SSO, the default chain is used
// unchanged and nothing is logged. This follows the aws CLI order, so a
// profile that also has credential_process, or a container credentials URI
// in the environment, still prefers SSO.
//
// The warning is written once per process to the logger's Out writer
// (os.Stderr by default), never to stdout, which carries machine-readable
// output such as holodeck list -o json.
//
// LoadConfig never retrieves credentials; they stay lazy.
func LoadConfig(ctx context.Context, log *logger.FunLogger, region string) (aws.Config, error) {
	env, err := config.NewEnvConfig()
	if err != nil {
		// LoadDefaultConfig parses the same environment and fails the same way.
		return config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}

	// Resolve the shared file paths now, not from the SDK's
	// DefaultSharedConfigFiles, which are computed once at package init.
	// For a real process both give the same paths; resolving here keeps the
	// profile check and the load below on the same files.
	configFile := env.SharedConfigFile
	if configFile == "" {
		configFile = config.DefaultSharedConfigFilename()
	}
	explicitCredsFile := env.SharedCredentialsFile != ""
	credsFile := env.SharedCredentialsFile
	if !explicitCredsFile {
		credsFile = config.DefaultSharedCredentialsFilename()
	}

	if !explicitCredsFile {
		if profile, ok := ssoShadowedByStaticKeys(ctx, env, configFile, credsFile); ok {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region),
				config.WithSharedConfigFiles([]string{configFile}),
				// An empty, non-nil list reads no credentials file at all.
				config.WithSharedCredentialsFiles([]string{}))
			if err == nil && aws.IsCredentialsProvider(cfg.Credentials, (*ssocreds.Provider)(nil)) {
				preferSSOWarnOnce.Do(func() { warnPreferSSO(log, profile) })
				return cfg, nil
			}
		}
	}

	return config.LoadDefaultConfig(ctx, config.WithRegion(region),
		config.WithSharedConfigFiles([]string{configFile}),
		config.WithSharedCredentialsFiles([]string{credsFile}))
}

// ssoShadowedByStaticKeys reports whether the active profile has both static
// keys and an SSO configuration and assumes no role, with no static keys in
// the environment. It returns the profile name. Any error reading the
// profile returns false.
func ssoShadowedByStaticKeys(ctx context.Context, env config.EnvConfig, configFile, credsFile string) (string, bool) {
	if env.Credentials.HasKeys() {
		return "", false
	}
	profile := env.SharedConfigProfile // AWS_PROFILE, else AWS_DEFAULT_PROFILE
	if profile == "" {
		profile = "default"
	}
	sc, err := config.LoadSharedConfigProfile(ctx, profile, func(o *config.LoadSharedConfigOptions) {
		o.ConfigFiles = []string{configFile}
		o.CredentialsFiles = []string{credsFile}
	})
	if err != nil {
		return "", false
	}
	if !sc.Credentials.HasKeys() || !hasSSOConfiguration(sc) || sc.RoleARN != "" {
		return "", false
	}
	return profile, true
}

// warnPreferSSO writes the prefer-SSO warning to the logger's Out writer,
// or os.Stderr without a logger. Unlike FunLogger.Info it ignores quiet
// mode: the user's credentials changed.
func warnPreferSSO(log *logger.FunLogger, profile string) {
	var w io.Writer = os.Stderr
	if log != nil && log.Out != nil {
		w = log.Out
	}
	fmt.Fprintf(w, "warning: "+preferSSOWarning+"\n", profile) //nolint:errcheck // best-effort warning
}

// hasSSOConfiguration mirrors the SDK's unexported
// (*config.SharedConfig).hasSSOConfiguration in config v1.33.13:
// hasSSOTokenProviderConfiguration() || hasLegacySSOConfiguration().
func hasSSOConfiguration(c config.SharedConfig) bool {
	return len(c.SSOSessionName) > 0 ||
		len(c.SSORegion) > 0 || len(c.SSOAccountID) > 0 || len(c.SSOStartURL) > 0 || len(c.SSORoleName) > 0
}
