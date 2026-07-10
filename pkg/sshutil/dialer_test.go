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
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

// countingTransport dials a fixed (unreachable) address, counting attempts.
type countingTransport struct {
	addr  string
	calls atomic.Int32
}

func (c *countingTransport) DialContext(ctx context.Context) (net.Conn, error) {
	c.calls.Add(1)
	d := net.Dialer{Timeout: 100 * time.Millisecond}
	return d.DialContext(ctx, "tcp", c.addr)
}
func (c *countingTransport) Target() string { return c.addr }
func (c *countingTransport) Close() error   { return nil }

// TestDialer_DefaultEnvelope guards the legacy dial envelope (20 attempts x
// 1s delay, 15s handshake, 30s keepalive) against silent drift. Each
// assertion compares the exported constant to an independently-written
// literal, never to itself, so a mutation to any one constant's value
// reddens this test; comparing a constant to itself would stay green
// regardless of the mutation.
func TestDialer_DefaultEnvelope(t *testing.T) {
	assert.Equal(t, 20, DefaultMaxAttempts, "legacy default retry attempts")
	assert.Equal(t, time.Second, DefaultRetryDelay, "legacy default retry delay")
	assert.Equal(t, 15*time.Second, DefaultHandshakeTimeout, "legacy default handshake timeout")
	assert.Equal(t, 30*time.Second, DefaultKeepaliveInterval, "legacy default keepalive interval")
}

func TestDialer_NilTransport_DirectDial(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU known_hosts
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("pong\n"))

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	client, err := d.Dial(context.Background(), srv.Addr(), nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "pong\n", string(out))
}

func TestDialer_RetryCount(t *testing.T) {
	tr := &countingTransport{addr: "127.0.0.1:1"} // nothing listening
	d := &Dialer{
		Auth:  AuthConfig{User: "u", KeyPath: mustKey(t)},
		Retry: RetryPolicy{MaxAttempts: 3, Delay: time.Millisecond},
		Log:   logger.NewLogger(),
	}
	_, err := d.Dial(context.Background(), "unused", tr)
	require.Error(t, err)
	assert.Equal(t, int32(3), tr.calls.Load(), "must attempt exactly MaxAttempts times")
}

func TestDialer_HandshakeTimeout(t *testing.T) {
	// Black-hole listener: accepts TCP, never sends the SSH banner. Without a
	// handshake deadline, ssh.NewClientConn blocks forever and this test hangs.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()

	d := &Dialer{
		Auth:     AuthConfig{User: "u", KeyPath: mustKey(t)},
		HostKey:  HostKeyPolicyOff,
		Retry:    RetryPolicy{MaxAttempts: 2, Delay: time.Millisecond},
		Timeouts: TimeoutConfig{Handshake: 200 * time.Millisecond},
		Log:      logger.NewLogger(),
	}
	start := time.Now()
	_, err = d.Dial(context.Background(), ln.Addr().String(), nil)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second, "handshake deadline must fire, not block")
}

func TestDialer_ContextCancel(t *testing.T) {
	tr := &countingTransport{addr: "127.0.0.1:1"}
	d := &Dialer{
		Auth:  AuthConfig{User: "u", KeyPath: mustKey(t)},
		Retry: RetryPolicy{MaxAttempts: 20, Delay: time.Second}, // long delay
		Log:   logger.NewLogger(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	_, err := d.Dial(ctx, "unused", tr)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), tr.calls.Load(), "cancel during retry delay stops after first attempt")
}

func TestDialer_StartsKeepalive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)

	d := &Dialer{
		Auth:     AuthConfig{User: "u", KeyPath: keyPath},
		HostKey:  HostKeyPolicyAcceptNew,
		Retry:    RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Timeouts: TimeoutConfig{Keepalive: 40 * time.Millisecond},
		Log:      logger.NewLogger(),
	}
	client, err := d.Dial(context.Background(), srv.Addr(), nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	require.Eventually(t, func() bool { return srv.Keepalives() >= 2 },
		2*time.Second, 20*time.Millisecond, "keepalive goroutine must send periodic requests")
}

// mustKey writes a throwaway client key and returns its path.
func mustKey(t *testing.T) string {
	t.Helper()
	p, _ := sshtest.GenerateKey(t)
	return p
}
