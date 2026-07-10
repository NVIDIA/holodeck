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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil"
)

// TestDialerFromSSHConfig proves dialerFromSSHConfig maps every
// v1alpha1.SSHConfig field onto the returned sshutil.Dialer. Neutering the
// entire non-nil branch — including the security-relevant
// KnownHostsPolicy->HostKey mapping — previously left the provisioner suite
// green; TestTransportFromSSHConfig_Bastion only exercises the bastion hop-1
// Dialer, not this function.
func TestDialerFromSSHConfig(t *testing.T) {
	const (
		keyPath  = "/keys/target"
		userName = "tester"
	)
	log := logger.NewLogger()

	populated := func(policy string) *v1alpha1.SSHConfig {
		return &v1alpha1.SSHConfig{
			KnownHostsPolicy:  policy,
			UseAgent:          true,
			AgentSocket:       "/run/agent.sock",
			MaxRetries:        7,
			HandshakeTimeout:  metav1.Duration{Duration: 42 * time.Second},
			KeepaliveInterval: metav1.Duration{Duration: 77 * time.Second},
		}
	}

	tests := []struct {
		name string
		cfg  *v1alpha1.SSHConfig
		want *sshutil.Dialer
	}{
		{
			// nil cfg must leave Retry/Timeouts at the Go zero value so
			// sshutil.Dial applies its own envelope (20 attempts x 1s delay,
			// 15s handshake, 30s keepalive — guarded independently by
			// TestDialer_DefaultEnvelope in pkg/sshutil/dialer_test.go). A
			// non-zero literal here would mean the legacy default moved into
			// the provisioner and silently diverged from sshutil's.
			name: "nil cfg defers retry and timeout fields to sshutil's defaults",
			cfg:  nil,
			want: &sshutil.Dialer{
				Auth:    sshutil.AuthConfig{User: userName, KeyPath: keyPath},
				HostKey: sshutil.HostKeyPolicyAcceptNew,
			},
		},
		{
			name: "accept-new policy maps every field",
			cfg:  populated("accept-new"),
			want: &sshutil.Dialer{
				Auth: sshutil.AuthConfig{
					User: userName, KeyPath: keyPath,
					UseAgent: true, AgentSocket: "/run/agent.sock",
				},
				HostKey:  sshutil.HostKeyPolicyAcceptNew,
				Retry:    sshutil.RetryPolicy{MaxAttempts: 7},
				Timeouts: sshutil.TimeoutConfig{Handshake: 42 * time.Second, Keepalive: 77 * time.Second},
			},
		},
		{
			name: "strict policy maps every field",
			cfg:  populated("strict"),
			want: &sshutil.Dialer{
				Auth: sshutil.AuthConfig{
					User: userName, KeyPath: keyPath,
					UseAgent: true, AgentSocket: "/run/agent.sock",
				},
				HostKey:  sshutil.HostKeyPolicyStrict,
				Retry:    sshutil.RetryPolicy{MaxAttempts: 7},
				Timeouts: sshutil.TimeoutConfig{Handshake: 42 * time.Second, Keepalive: 77 * time.Second},
			},
		},
		{
			name: "off policy maps every field",
			cfg:  populated("off"),
			want: &sshutil.Dialer{
				Auth: sshutil.AuthConfig{
					User: userName, KeyPath: keyPath,
					UseAgent: true, AgentSocket: "/run/agent.sock",
				},
				HostKey:  sshutil.HostKeyPolicyOff,
				Retry:    sshutil.RetryPolicy{MaxAttempts: 7},
				Timeouts: sshutil.TimeoutConfig{Handshake: 42 * time.Second, Keepalive: 77 * time.Second},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialerFromSSHConfig(keyPath, userName, tt.cfg, log)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Auth, got.Auth, "Auth (User/KeyPath/UseAgent/AgentSocket)")
			assert.Equal(t, tt.want.HostKey, got.HostKey, "KnownHostsPolicy->HostKey")
			assert.Equal(t, tt.want.Retry, got.Retry, "MaxRetries->Retry.MaxAttempts")
			assert.Equal(t, tt.want.Timeouts, got.Timeouts, "HandshakeTimeout/KeepaliveInterval->Timeouts")
		})
	}
}

// countingTransport counts DialContext() calls while connecting to a black hole.
type countingTransport struct {
	addr  string
	calls atomic.Int32
}

func (t *countingTransport) DialContext(ctx context.Context) (net.Conn, error) {
	t.calls.Add(1)
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.DialContext(ctx, "tcp", t.addr)
}

func (t *countingTransport) Target() string { return t.addr }
func (t *countingTransport) Close() error   { return nil }

// TestNew_HandshakeTimeout verifies that New() (via the sshutil.Dialer default
// envelope) configures an SSH handshake timeout. Without the timeout,
// ssh.NewClientConn blocks forever against a host that accepts TCP but never
// responds with the SSH banner. With the default 15s handshake, each attempt
// fails in ~15s.
//
// We verify this by connecting to a black hole server and checking that
// multiple retry attempts complete within a bounded time (proving the
// handshake timeout fires instead of blocking indefinitely).
func TestNew_HandshakeTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: test waits for SSH handshake timeouts")
	}

	// Start a TCP listener that never performs the SSH handshake
	addr := startBlackHoleServer(t)
	keyPath := writeTestSSHKey(t)
	log := logger.NewLogger()

	transport := &countingTransport{addr: addr}

	// Run New() in a goroutine — it will retry up to the default 20 attempts,
	// each timing out at the default 15s handshake. We don't want to wait for
	// all 20 retries (~5 min), so we observe progress from the outside.
	errCh := make(chan error, 1)
	go func() {
		_, err := New(log, keyPath, "testuser", "black-hole-host", WithTransport(transport))
		errCh <- err
	}()

	// Wait for at least 2 Dial() calls — proving that:
	// 1. The first SSH handshake attempt timed out (didn't block forever)
	// 2. The retry logic moved on to attempt #2
	deadline := time.After(45 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			calls := transport.calls.Load()
			t.Fatalf("timed out waiting for retry progress; only %d Dial() calls observed — "+
				"handshake timeout may not be configured", calls)
		case <-ticker.C:
			if transport.calls.Load() >= 2 {
				// Success: at least 2 attempts means the first one timed out
				// and the retry loop continued. The timeout is working.
				return
			}
		case err := <-errCh:
			// New() returned — all retries exhausted
			if err == nil {
				t.Fatal("expected connection error, got nil")
			}
			calls := transport.calls.Load()
			if calls < 2 {
				t.Fatalf("New() returned after only %d Dial() calls — "+
					"expected multiple retries with handshake timeouts", calls)
			}
			return
		}
	}
}

// startBlackHoleServer starts a TCP listener that accepts connections and
// holds them open indefinitely without performing an SSH handshake.
func startBlackHoleServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				<-done
				_ = c.Close()
			}(conn)
		}
	}()

	t.Cleanup(func() {
		close(done)
		_ = listener.Close()
	})

	return listener.Addr().String()
}

// writeTestSSHKey creates a temporary ed25519 SSH private key in PEM format.
func writeTestSSHKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	pemBytes := pem.EncodeToMemory(pemBlock)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}
	return keyPath
}
