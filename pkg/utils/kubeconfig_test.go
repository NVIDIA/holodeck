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

package utils

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestRewriteKubeConfigServer(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		serverURL string
		expected  string
	}{
		{
			name: "rewrite private IP to public IP",
			input: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: dGVzdA==
    server: https://10.0.0.1:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
users:
- name: kubernetes-admin
  user:
    client-certificate-data: dGVzdA==
    client-key-data: dGVzdA==
`,
			serverURL: "https://54.1.2.3:6443",
			expected:  "https://54.1.2.3:6443",
		},
		{
			name: "rewrite to NLB DNS",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://10.0.0.1:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: admin
  name: admin@kubernetes
current-context: admin@kubernetes
kind: Config
users:
- name: admin
  user:
    client-certificate-data: dGVzdA==
`,
			serverURL: "https://my-nlb.elb.amazonaws.com:6443",
			expected:  "https://my-nlb.elb.amazonaws.com:6443",
		},
		{
			name:      "empty server URL is no-op",
			input:     "apiVersion: v1\nclusters:\n- cluster:\n    server: https://10.0.0.1:6443\n  name: kubernetes\nkind: Config\n",
			serverURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "kubeconfig")
			err := os.WriteFile(path, []byte(tt.input), 0600)
			require.NoError(t, err)

			err = RewriteKubeConfigServer(path, tt.serverURL)
			require.NoError(t, err)

			data, err := os.ReadFile(path) //nolint:gosec // test file from t.TempDir()
			require.NoError(t, err)

			if tt.serverURL == "" {
				assert.Contains(t, string(data), "https://10.0.0.1:6443")
			} else {
				assert.Contains(t, string(data), tt.expected)
				assert.NotContains(t, string(data), "10.0.0.1")
			}
		})
	}
}

func TestApplyRemoteAccess(t *testing.T) {
	const validInput = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: dGVzdA==
    server: https://10.0.0.1:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
users:
- name: kubernetes-admin
  user:
    client-certificate-data: dGVzdA==
    client-key-data: dGVzdA==
`

	tests := []struct {
		name           string
		remoteAccess   bool
		hostUrl        string
		initialMode    os.FileMode
		initialContent string
		writeFile      bool
		// expectations
		wantErr        bool
		wantErrIs      []error // sentinels to assert via errors.Is
		wantBytesEqual bool    // file unchanged from initialContent
		wantScheme     string  // "" = skip URL parse assertion
		wantHostname   string
		wantPort       string
		wantMode       os.FileMode
	}{
		{
			name:           "disabled no-op",
			remoteAccess:   false,
			hostUrl:        "ec2-1-2-3-4.compute.amazonaws.com",
			initialMode:    0o600,
			initialContent: validInput,
			writeFile:      true,
			wantBytesEqual: true,
			wantMode:       0o600,
		},
		{
			name:           "enabled valid",
			remoteAccess:   true,
			hostUrl:        "ec2-1-2-3-4.compute.amazonaws.com",
			initialMode:    0o600,
			initialContent: validInput,
			writeFile:      true,
			wantScheme:     "https",
			wantHostname:   "ec2-1-2-3-4.compute.amazonaws.com",
			wantPort:       "6443",
			wantMode:       0o600,
		},
		{
			name:           "enabled IPv4 host",
			remoteAccess:   true,
			hostUrl:        "54.1.2.3",
			initialMode:    0o600,
			initialContent: validInput,
			writeFile:      true,
			wantScheme:     "https",
			wantHostname:   "54.1.2.3",
			wantPort:       "6443",
			wantMode:       0o600,
		},
		{
			name:           "enabled hostUrl empty",
			remoteAccess:   true,
			hostUrl:        "",
			initialMode:    0o600,
			initialContent: validInput,
			writeFile:      true,
			wantErr:        true,
			wantBytesEqual: true,
			wantMode:       0o600,
		},
		{
			name:         "enabled missing file",
			remoteAccess: true,
			hostUrl:      "host.example.com",
			writeFile:    false,
			wantErr:      true,
			wantErrIs:    []error{ErrRewriteFailed, os.ErrNotExist},
		},
		{
			name:           "enabled malformed YAML",
			remoteAccess:   true,
			hostUrl:        "host.example.com",
			initialMode:    0o600,
			initialContent: "this is not yaml: [unclosed",
			writeFile:      true,
			wantErr:        true,
			wantErrIs:      []error{ErrRewriteFailed},
			wantBytesEqual: true,
			wantMode:       0o600,
		},
		{
			name:         "enabled URL already correct (idempotent)",
			remoteAccess: true,
			hostUrl:      "host.example.com",
			initialMode:  0o600,
			initialContent: `apiVersion: v1
clusters:
- cluster:
    server: https://host.example.com:6443
  name: kubernetes
kind: Config
`,
			writeFile:    true,
			wantScheme:   "https",
			wantHostname: "host.example.com",
			wantPort:     "6443",
			wantMode:     0o600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "kubeconfig")

			var initialBytes []byte
			if tt.writeFile {
				initialBytes = []byte(tt.initialContent)
				require.NoError(t, os.WriteFile(path, initialBytes, tt.initialMode))
			}

			cfg := &v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Kubernetes: v1alpha1.Kubernetes{
						RemoteAccess: tt.remoteAccess,
					},
				},
			}

			err := ApplyRemoteAccess(cfg, tt.hostUrl, path)

			if tt.wantErr {
				require.Error(t, err)
				for _, target := range tt.wantErrIs {
					assert.ErrorIs(t, err, target, "err must wrap %v", target)
				}
			} else {
				require.NoError(t, err)
			}

			// File-state assertions only apply when the file existed.
			if !tt.writeFile {
				return
			}

			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			if tt.wantMode != 0 {
				assert.Equal(t, tt.wantMode, info.Mode().Perm(),
					"file mode must equal %o", tt.wantMode)
			}

			data, readErr := os.ReadFile(path) //nolint:gosec // test temp file
			require.NoError(t, readErr)

			if tt.wantBytesEqual {
				assert.True(t, bytes.Equal(initialBytes, data),
					"file bytes must be unchanged")
			}

			if tt.wantScheme != "" {
				var kc kubeConfig
				require.NoError(t, yaml.Unmarshal(data, &kc),
					"output kubeconfig must be valid YAML")
				require.NotEmpty(t, kc.Clusters,
					"output kubeconfig must have at least one cluster")
				u, parseErr := url.Parse(kc.Clusters[0].Cluster.Server)
				require.NoError(t, parseErr,
					"server URL must be parseable: %q", kc.Clusters[0].Cluster.Server)
				assert.Equal(t, tt.wantScheme, u.Scheme)
				assert.Equal(t, tt.wantHostname, u.Hostname())
				assert.Equal(t, tt.wantPort, u.Port())
			}
		})
	}
}

// TestGetKubeConfig_StrictPolicyReachesDialer is the B1 proving test. It drives a
// real production provisioner.New call site (GetKubeConfig -> provisioner.New)
// and asserts that a strict knownHostsPolicy set on the environment's
// auth.sshConfig actually reaches the Dialer.
//
// With a fresh (empty) known_hosts, strict must REJECT the unknown in-process
// server. If WithSSHConfig were not wired at the call site, the dial would
// default to accept-new and SUCCEED — so this test goes RED the moment the
// option is dropped from GetKubeConfig, discriminating exactly the
// end-to-end-inert regression B1 fixed. MaxRetries:1 keeps it fast (a strict
// rejection is not transient, so retrying is pointless).
func TestGetKubeConfig_StrictPolicyReachesDialer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)           // fresh known_hosts: the server is unknown (macOS path)
	t.Setenv("XDG_CACHE_HOME", dir) // Linux TOFU path also honors this

	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)

	cfg := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Auth: v1alpha1.Auth{
				PrivateKey: keyPath,
				Username:   "tester",
				SSHConfig: &v1alpha1.SSHConfig{
					KnownHostsPolicy: "strict",
					MaxRetries:       1,
				},
			},
		},
	}

	err := GetKubeConfig(logger.NewLogger(), cfg, srv.Addr(), filepath.Join(t.TempDir(), "kubeconfig"))
	require.Error(t, err, "strict policy against an unknown host must reject the dial")
	assert.Contains(t, err.Error(), "strict",
		"the strict host-key policy must reach the Dialer through the production New call site")
}
