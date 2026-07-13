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

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/pkg/jyaml"
)

func TestSSHConfig_RoundTrip(t *testing.T) {
	in := SSHConfig{
		Bastion:           &BastionConfig{Host: "bastion.example.com", Username: "ec2-user", PrivateKey: "/keys/bastion.pem"},
		UseAgent:          true,
		AgentSocket:       "/tmp/agent.sock",
		KnownHostsPolicy:  "strict",
		ConnectTimeout:    metav1.Duration{Duration: 30 * time.Second},
		HandshakeTimeout:  metav1.Duration{Duration: 15 * time.Second},
		KeepaliveInterval: metav1.Duration{Duration: 30 * time.Second},
		MaxRetries:        20,
	}
	y, err := jyaml.MarshalYAML(in)
	require.NoError(t, err)
	out, err := jyaml.Unmarshal[SSHConfig](y)
	require.NoError(t, err)
	assert.Equal(t, in, out, "sshConfig must survive a YAML round-trip")
}

func TestSSHConfig_Validate(t *testing.T) {
	require.NoError(t, (*SSHConfig)(nil).Validate()) // nil-safe: callers pass a possibly-nil field
	require.NoError(t, (&SSHConfig{KnownHostsPolicy: "accept-new"}).Validate())
	require.NoError(t, (&SSHConfig{}).Validate()) // all-optional
	assert.Error(t, (&SSHConfig{KnownHostsPolicy: "yolo"}).Validate())
	assert.Error(t, (&SSHConfig{MaxRetries: -1}).Validate())
	assert.Error(t, (&SSHConfig{Bastion: &BastionConfig{Host: ""}}).Validate())
}

// TestAuth_BackCompat_NoSSHConfig guards the #1 CRD back-compat bug: an
// env.yaml written before sshConfig existed must unmarshal into Auth with
// identical field values and a nil SSHConfig, and re-marshaling that Auth
// must not introduce an "sshConfig" key. A missing "omitempty" on the new
// field would both break this and leak an empty block into every cached
// env.yaml (create/update/get/status/describe/ssh/scp all round-trip Auth
// through the on-disk cache file).
func TestAuth_BackCompat_NoSSHConfig(t *testing.T) {
	legacy := []byte(`
keyName: cnt-ci
username: ubuntu
publicKey: /home/runner/.ssh/id_rsa.pub
privateKey: /home/runner/.cache/key
`)

	got, err := jyaml.Unmarshal[Auth](legacy)
	require.NoError(t, err)

	want := Auth{
		KeyName:    "cnt-ci",
		Username:   "ubuntu",
		PublicKey:  "/home/runner/.ssh/id_rsa.pub",
		PrivateKey: "/home/runner/.cache/key",
	}
	assert.Equal(t, want, got, "legacy auth block without sshConfig must unmarshal identically to today")
	assert.Nil(t, got.SSHConfig, "sshConfig must stay nil when omitted from the YAML")

	out, err := jyaml.MarshalYAML(want)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "sshConfig", "omitempty must drop sshConfig from marshaled output when unset")
}

func TestSSHConfig_DeepCopy_Aliasing(t *testing.T) {
	orig := &SSHConfig{Bastion: &BastionConfig{Host: "b.example.com", Username: "ec2-user"}}
	cp := orig.DeepCopy()
	require.NotSame(t, orig.Bastion, cp.Bastion, "Bastion must be a fresh pointer")
	orig.Bastion.Host = "MUTATED"
	assert.Equal(t, "b.example.com", cp.Bastion.Host, "copy must not observe original's mutation")
}

func TestAuth_DeepCopy_SSHConfigDeep(t *testing.T) {
	orig := &Auth{SSHConfig: &SSHConfig{Bastion: &BastionConfig{Host: "b"}}}
	cp := orig.DeepCopy()
	require.NotSame(t, orig.SSHConfig, cp.SSHConfig)
	require.NotSame(t, orig.SSHConfig.Bastion, cp.SSHConfig.Bastion)
	orig.SSHConfig.Bastion.Host = "MUT"
	assert.Equal(t, "b", cp.SSHConfig.Bastion.Host)
}
