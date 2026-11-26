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

package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
)

// ParentHomeEnv carries the HOME of the process that called RunAWSIsolated,
// so the guard can check that no error names a file under it.
const ParentHomeEnv = "HOLODECK_TEST_PARENT_HOME"

// isolatedEndpoint is a closed local port: if anything still tried to send an
// AWS request, it would be refused locally instead of reaching AWS.
const isolatedEndpoint = "http://127.0.0.1:9"

// AWSCredentialEnv lists the environment variables RunAWSIsolated removes:
// every variable the AWS SDK reads to pick a profile or find credentials.
func AWSCredentialEnv() []string {
	return []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_ACCESS_KEY", "AWS_SECRET_KEY",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"AWS_CONTAINER_AUTHORIZATION_TOKEN",
	}
}

// RunAWSIsolated runs the package's tests in a child test process that cannot
// reach real AWS credentials. Use it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.RunAWSIsolated(m)) }
//
// The child starts with HOME set to a new temp dir, AWS_CONFIG_FILE and
// AWS_SHARED_CREDENTIALS_FILE set to empty files in it, every variable from
// AWSCredentialEnv removed, IMDS disabled, and AWS_ENDPOINT_URL set to a
// closed local port. Because the child is a new process, the AWS SDK's
// package-init default paths (config.DefaultSharedConfigFiles and
// DefaultSharedCredentialsFiles, computed from HOME before TestMain runs) also
// point into the temp HOME. Setting the variables in this process would be
// too late for them.
//
// In the child it just runs the tests. It returns the exit code to pass to
// os.Exit.
func RunAWSIsolated(m *testing.M) int {
	if os.Getenv(IsolatedHomeEnv) != "" {
		return m.Run()
	}

	home, err := os.MkdirTemp("", "holodeck-aws-isolated-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil.RunAWSIsolated: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(home) }() //nolint:errcheck // best-effort temp cleanup

	// Outside home/.aws, which IsolatedHome resets between tests.
	dir := filepath.Join(home, "isolated-aws")
	configFile := filepath.Join(dir, "config")
	credsFile := filepath.Join(dir, "credentials")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "testutil.RunAWSIsolated: %v\n", err)
		return 1
	}
	for _, f := range []string{configFile, credsFile} {
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "testutil.RunAWSIsolated: %v\n", err)
			return 1
		}
	}

	set := map[string]string{
		"HOME":                        home,
		IsolatedHomeEnv:               home,
		ParentHomeEnv:                 os.Getenv("HOME"),
		"AWS_CONFIG_FILE":             configFile,
		"AWS_SHARED_CREDENTIALS_FILE": credsFile,
		"AWS_EC2_METADATA_DISABLED":   "true",
		"AWS_ENDPOINT_URL":            isolatedEndpoint,
	}
	drop := map[string]bool{}
	for _, k := range AWSCredentialEnv() {
		drop[k] = true
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, overridden := set[k]; !overridden && !drop[k] {
			env = append(env, kv)
		}
	}
	for k, v := range set {
		env = append(env, k+"="+v)
	}

	//nolint:gosec // re-executes this same test binary with its own arguments
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil.RunAWSIsolated: %v\n", err)
		return 1
	}
	return 0
}

// AWSIsolationProblems lists every way the current process is not isolated
// the way RunAWSIsolated isolates it. An empty result means isolated. It makes
// no AWS SDK call.
func AWSIsolationProblems() []string {
	var problems []string
	home := os.Getenv("HOME")
	marker := os.Getenv(IsolatedHomeEnv)
	if marker == "" {
		problems = append(problems,
			IsolatedHomeEnv+" is not set: the process was not started by testutil.RunAWSIsolated")
	}
	if home != marker {
		problems = append(problems, fmt.Sprintf("HOME=%q is not the isolated HOME %q", home, marker))
	}
	if !within(os.TempDir(), home) {
		problems = append(problems, fmt.Sprintf("HOME=%q is not inside os.TempDir() %q", home, os.TempDir()))
	}
	for _, k := range []string{"AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE"} {
		if v := os.Getenv(k); !within(home, v) || !isEmptyFile(v) {
			problems = append(problems, fmt.Sprintf("%s=%q is not an empty file inside HOME", k, v))
		}
	}
	for _, k := range AWSCredentialEnv() {
		if _, ok := os.LookupEnv(k); ok {
			problems = append(problems, k+" is set")
		}
	}
	if os.Getenv("AWS_EC2_METADATA_DISABLED") != "true" {
		problems = append(problems, `AWS_EC2_METADATA_DISABLED is not "true"`)
	}
	if os.Getenv("AWS_ENDPOINT_URL") != isolatedEndpoint {
		problems = append(problems, fmt.Sprintf("AWS_ENDPOINT_URL is not %q", isolatedEndpoint))
	}
	if p := config.DefaultSharedConfigFiles[0]; !within(home, p) {
		problems = append(problems, fmt.Sprintf(
			"SDK default config file %q (computed at package init) is outside the isolated HOME", p))
	}
	if p := config.DefaultSharedCredentialsFiles[0]; !within(home, p) {
		problems = append(problems, fmt.Sprintf(
			"SDK default credentials file %q (computed at package init) is outside the isolated HOME", p))
	}
	return problems
}

// RequireAWSIsolation fails t unless the process is isolated from real AWS
// credentials. It checks the environment and the SDK's package-init paths
// first, without touching the SDK, and stops there if anything is wrong.
// Only then does it load the default SDK config and require that resolving
// credentials fails without naming a file under the parent's HOME.
func RequireAWSIsolation(t *testing.T) {
	t.Helper()
	if problems := AWSIsolationProblems(); len(problems) > 0 {
		t.Fatalf("AWS isolation is not active (add TestMain calling testutil.RunAWSIsolated):\n- %s",
			strings.Join(problems, "\n- "))
	}
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("load default AWS config in the isolated process: %v", err)
	}
	_, err = cfg.Credentials.Retrieve(ctx)
	if err == nil {
		t.Fatalf("AWS credentials resolved inside the isolated process")
	}
	if parent := os.Getenv(ParentHomeEnv); parent != "" && strings.Contains(err.Error(), parent) {
		t.Fatalf("credential error names a file under the real HOME %q: %v", parent, err)
	}
}

// within reports whether path is dir or inside it. An empty dir or path is
// never within.
func within(dir, path string) bool {
	if dir == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isEmptyFile reports whether path is an existing, empty regular file.
func isEmptyFile(path string) bool {
	fi, err := os.Stat(path) //nolint:gosec // G703: read-only stat of a path the guard is checking
	return err == nil && fi.Mode().IsRegular() && fi.Size() == 0
}
