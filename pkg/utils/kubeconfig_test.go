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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			data, err := os.ReadFile(path)
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
