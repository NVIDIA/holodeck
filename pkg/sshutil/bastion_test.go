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

package sshutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestBastionTransport_TwoHop(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate per-hop TOFU
	keyPath, pub := sshtest.GenerateKey(t)
	target := sshtest.NewServer(t, pub, sshtest.WithExecOutput("hello-from-target\n"))
	bastion := sshtest.NewServer(t, pub, sshtest.WithForwarding())

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	bt := &BastionTransport{Bastion: bastion.Addr(), TargetHost: target.Addr(), Dialer: d}
	defer func() { _ = bt.Close() }()

	client, err := d.Dial(context.Background(), target.Addr(), bt)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Prove the chain reaches the TARGET: scripted exec output round-trips.
	sess, err := client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "hello-from-target\n", string(out))

	// Prove the BASTION forwarded the hop, and Close() releases hop-1 cleanly.
	assert.GreaterOrEqual(t, bastion.Forwards(), 1, "bastion must forward the direct-tcpip hop")
	assert.NoError(t, bt.Close())
}

// TestBastionTransport_BastionHostKey_RecordedOnFirstConnect guards the T2
// carry-forward: hop-1 (the bastion) must go through the Dialer's TOFU
// host-key path, not be skipped or dialed with InsecureIgnoreHostKey. If
// BastionTransport ever dialed hop-1 without routing through b.Dialer.Dial,
// the bastion's address would never appear in known_hosts and this
// assertion would go red.
func TestBastionTransport_BastionHostKey_RecordedOnFirstConnect(t *testing.T) {
	path := setupTOFUTest(t)
	keyPath, pub := sshtest.GenerateKey(t)
	target := sshtest.NewServer(t, pub, sshtest.WithExecOutput("hello-from-target\n"))
	bastion := sshtest.NewServer(t, pub, sshtest.WithForwarding())

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	bt := &BastionTransport{Bastion: bastion.Addr(), TargetHost: target.Addr(), Dialer: d}
	defer func() { _ = bt.Close() }()

	client, err := d.Dial(context.Background(), target.Addr(), bt)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	data, err := os.ReadFile(path) //nolint:gosec // path from setupTOFUTest's isolated tmpdir
	require.NoError(t, err)

	bastionEntry := knownhosts.Normalize(bastion.Addr())
	assert.Contains(t, string(data), bastionEntry,
		"bastion hop-1 host key must be recorded via the Dialer's TOFU path")
}

// TestBastionTransport_BastionHostKey_MismatchRejected guards the same
// carry-forward from the rejection side: a changed/MITM'd bastion host key
// must fail hop-1 verification before the tunnel ever reaches the target.
// If BastionTransport routed hop-1 host-key handling around the Dialer
// (e.g. ssh.InsecureIgnoreHostKey or a policy=off shortcut), this dial
// would succeed and bastion.Forwards() would be nonzero.
func TestBastionTransport_BastionHostKey_MismatchRejected(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	keyPath, pub := sshtest.GenerateKey(t)
	target := sshtest.NewServer(t, pub, sshtest.WithExecOutput("hello-from-target\n"))
	bastion := sshtest.NewServer(t, pub, sshtest.WithForwarding())

	// Seed a known_hosts entry for the bastion's address bound to an
	// unrelated key, simulating a host key that changed since it was first
	// trusted.
	wrongKey := generateTestKey(t)
	line := knownhosts.Line([]string{knownhosts.Normalize(bastion.Addr())}, wrongKey)
	require.NoError(t, os.WriteFile(path, []byte(line+"\n"), 0600))

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 1, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	bt := &BastionTransport{Bastion: bastion.Addr(), TargetHost: target.Addr(), Dialer: d}
	defer func() { _ = bt.Close() }()

	_, err := d.Dial(context.Background(), target.Addr(), bt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bastion hop-1 dial", "must fail at hop-1 (bastion), not fall through to the target")
	assert.Contains(t, err.Error(), "host key mismatch", "must be rejected as a host-key mismatch, not some other failure")
	assert.Equal(t, 0, bastion.Forwards(), "hop-1 must fail before any direct-tcpip forward is requested")
}
