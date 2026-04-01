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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/NVIDIA/holodeck/internal/logger"
)

// countingTransport counts Dial() calls while connecting to a black hole.
type countingTransport struct {
	addr  string
	calls atomic.Int32
}

func (t *countingTransport) Dial() (net.Conn, error) {
	t.calls.Add(1)
	return net.DialTimeout("tcp", t.addr, 5*time.Second)
}

func (t *countingTransport) Target() string { return t.addr }
func (t *countingTransport) Close() error   { return nil }

// TestNew_HandshakeTimeout verifies that New() (via connectOrDie) configures
// an SSH handshake timeout. Without the timeout, ssh.NewClientConn blocks
// forever against a host that accepts TCP but never responds with the SSH
// banner. With the timeout, each attempt fails in ~15s.
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

	// Run New() in a goroutine — it will retry up to sshMaxRetries times,
	// each timing out in sshHandshakeTimeout (~15s). We don't want to wait
	// for all 20 retries (~5 min), so we observe progress from the outside.
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
