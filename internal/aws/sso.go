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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
)

// ssoCacheErrMarker appears in every error the SDK's SSO token provider
// and token-cache loader return (ssocreds/sso_token_provider.go and
// sso_cached_token.go). sso-session profiles fail through those paths with
// plain errors, not *ssocreds.InvalidTokenError, so the type alone misses
// them. TestSSOHintRealSDKChain fails if an SDK upgrade changes this text.
const ssoCacheErrMarker = "cached SSO token"

// SSOHint adds an "aws sso login" hint to errors caused by an expired,
// missing or revoked AWS SSO session: the typed InvalidTokenError (legacy
// profiles), the SSO service's UnauthorizedException (a token the portal no
// longer accepts), or the untyped token-cache errors of sso-session profiles.
// The profile named in the hint is the one the SDK used: AWS_PROFILE, else
// AWS_DEFAULT_PROFILE. Any other error, and nil, is
// returned unchanged. The original error stays in the chain.
func SSOHint(err error) error {
	if err == nil {
		return nil
	}
	var ssoErr *ssocreds.InvalidTokenError
	var unauthErr *ssotypes.UnauthorizedException
	if !errors.As(err, &ssoErr) && !errors.As(err, &unauthErr) &&
		!strings.Contains(err.Error(), ssoCacheErrMarker) {
		return err
	}
	p := os.Getenv("AWS_PROFILE")
	if p == "" {
		p = os.Getenv("AWS_DEFAULT_PROFILE")
	}
	if p != "" {
		return fmt.Errorf("AWS SSO session for profile %q is expired or missing; run \"aws sso login --profile %s\" and retry: %w", p, p, err)
	}
	return fmt.Errorf("AWS SSO session is expired or missing; run \"aws sso login\" and retry: %w", err)
}
