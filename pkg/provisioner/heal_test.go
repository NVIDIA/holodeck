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

package provisioner

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestProvisioner_EnsureClient_HealsDroppedConnection(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("ok\n"))

	p, err := New(logger.NewLogger(), keyPath, "tester", srv.Addr())
	require.NoError(t, err)
	require.NotNil(t, p.Client)

	// Simulate a mid-provisioning drop.
	require.NoError(t, p.Client.Close())

	// ensureClient must heal (re-dial) transparently.
	require.NoError(t, p.ensureClient(context.Background()))
	sess, err := p.Client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(out))

	// A live client is reused (same pointer) on the next ensureClient.
	before := p.Client
	require.NoError(t, p.ensureClient(context.Background()))
	assert.Same(t, before, p.Client, "live connection must be reused, not re-dialed")
}

// countingCloser wraps a Transport and counts Close() calls. It proves the
// provisioner owns the transport lifecycle (T3-critic carry-forward): a
// BastionTransport/SSMTransport must be torn down on both the success path
// (Provisioner.Close) and the error path (a failed dial in New), or hop-1 SSH
// clients and SSM session processes leak.
type countingCloser struct {
	inner  Transport
	closes atomic.Int32
}

func (c *countingCloser) DialContext(ctx context.Context) (net.Conn, error) {
	return c.inner.DialContext(ctx)
}
func (c *countingCloser) Target() string { return c.inner.Target() }
func (c *countingCloser) Close() error {
	c.closes.Add(1)
	return c.inner.Close()
}

// TestProvisioner_Close_ClosesTransport proves the success-path lifecycle:
// Provisioner.Close() closes the transport it acquired, not just the client.
func TestProvisioner_Close_ClosesTransport(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("ok\n"))

	ct := &countingCloser{inner: sshutil.NewDirectTransport(srv.Addr())}
	p, err := New(logger.NewLogger(), keyPath, "tester", srv.Addr(), WithTransport(ct))
	require.NoError(t, err)
	require.NotNil(t, p.Client)

	require.NoError(t, p.Close())
	assert.Equal(t, int32(1), ct.closes.Load(), "Provisioner.Close must close the owned transport")
}

// TestProvisioner_New_DialFailure_ClosesTransport proves the error-path
// lifecycle: when the initial dial in New fails, the transport is closed before
// New returns so no SSM/bastion resources leak. MaxRetries:1 keeps it fast; the
// dead port fails the dial immediately.
func TestProvisioner_New_DialFailure_ClosesTransport(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU
	keyPath, _ := sshtest.GenerateKey(t)

	ct := &countingCloser{inner: sshutil.NewDirectTransport("127.0.0.1:1")}
	_, err := New(logger.NewLogger(), keyPath, "tester", "127.0.0.1:1",
		WithTransport(ct), WithSSHConfig(&v1alpha1.SSHConfig{MaxRetries: 1}))
	require.Error(t, err)
	assert.Equal(t, int32(1), ct.closes.Load(), "New must close the transport when the dial fails")
}

// TestTransportFromSSHConfig_Bastion proves that a bastion SSHConfig selects a
// BastionTransport wired for a two-hop dial: hop-1 uses the bastion's own
// credentials and the configured host-key policy; hop-2 targets the node.
func TestTransportFromSSHConfig_Bastion(t *testing.T) {
	cfg := &v1alpha1.SSHConfig{
		Bastion: &v1alpha1.BastionConfig{
			Host:       "bastion.example:22",
			Username:   "jump",
			PrivateKey: "/keys/bastion",
		},
		KnownHostsPolicy: "strict",
	}
	tr := transportFromSSHConfig("10.0.0.5", "/keys/target", "tester", cfg, logger.NewLogger())

	bt, ok := tr.(*sshutil.BastionTransport)
	require.True(t, ok, "bastion config must select a BastionTransport")
	assert.Equal(t, "bastion.example:22", bt.Bastion)
	assert.Equal(t, "10.0.0.5", bt.TargetHost)
	require.NotNil(t, bt.Dialer)
	assert.Equal(t, "jump", bt.Dialer.Auth.User, "hop-1 uses the bastion username")
	assert.Equal(t, "/keys/bastion", bt.Dialer.Auth.KeyPath, "hop-1 uses the bastion key")
	assert.Equal(t, sshutil.HostKeyPolicyStrict, bt.Dialer.HostKey, "hop-1 honors the configured policy")
}

// TestTransportFromSSHConfig_Direct proves the default: no bastion (or nil
// config) selects a plain DirectTransport.
func TestTransportFromSSHConfig_Direct(t *testing.T) {
	tr := transportFromSSHConfig("10.0.0.5", "/keys/target", "tester", nil, logger.NewLogger())
	_, ok := tr.(*sshutil.DirectTransport)
	assert.True(t, ok, "nil config must select a DirectTransport")
}
