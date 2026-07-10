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

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

// TestRunProvision_StrictPolicyReachesDialer drives the production
// runProvision -> provisioner.New call site and asserts that a strict
// knownHostsPolicy set on the environment's auth.sshConfig actually reaches
// the Dialer. runProvision is unexported, but it is the exact method
// NewCommand's update Action reaches (via command.run) once reprovisioning
// starts, so this test exercises it directly rather than scaffolding the
// full CLI command plus an on-disk instance cache.
//
// With a fresh (empty) known_hosts, strict must REJECT the unknown in-process
// server. If WithSSHConfig were not wired at this call site, the dial would
// default to accept-new and SUCCEED — so this test goes RED the moment the
// option is dropped from runProvision. Mirrors
// TestGetKubeConfig_StrictPolicyReachesDialer in pkg/utils/kubeconfig_test.go,
// which proves the same contract at the GetKubeConfig call site.
func TestRunProvision_StrictPolicyReachesDialer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)           // fresh known_hosts: the server is unknown (macOS path)
	t.Setenv("XDG_CACHE_HOME", dir) // Linux TOFU path also honors this

	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)

	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderSSH,
			Auth: v1alpha1.Auth{
				PrivateKey: keyPath,
				Username:   "tester",
				SSHConfig: &v1alpha1.SSHConfig{
					KnownHostsPolicy: "strict",
					MaxRetries:       1,
				},
			},
			Instance: v1alpha1.Instance{
				HostUrl: srv.Addr(),
			},
		},
	}

	m := &command{log: logger.NewLogger()}
	err := m.runProvision(env)
	require.Error(t, err, "strict policy against an unknown host must reject the dial")
	assert.Contains(t, err.Error(), "strict",
		"the strict host-key policy must reach the Dialer through the production "+
			"runProvision -> provisioner.New call site")
}
