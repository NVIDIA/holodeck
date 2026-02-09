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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func generateTestKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to create SSH public key: %v", err)
	}
	return pubKey
}

// setupTOFUTest isolates the TOFU cache in a temp directory by overriding HOME.
// Returns the expected known_hosts path.
func setupTOFUTest(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir failed: %v", err)
	}
	return filepath.Join(cacheDir, "holodeck", "known_hosts")
}

func TestTOFU_FirstConnection_RecordsKey(t *testing.T) {
	knownHostsPath := setupTOFUTest(t)

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := tofuHostKeyCallback()
	if err := cb("testhost:22", addr, key); err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}

	data, err := os.ReadFile(knownHostsPath) // nolint:gosec // test helper with controlled tmpdir path
	if err != nil {
		t.Fatalf("known_hosts should exist at %s: %v", knownHostsPath, err)
	}
	if len(data) == 0 {
		t.Fatal("known_hosts should not be empty")
	}
}

func TestTOFU_SubsequentConnection_SameKey_Accepted(t *testing.T) {
	_ = setupTOFUTest(t)

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := tofuHostKeyCallback()

	if err := cb("testhost:22", addr, key); err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	if err := cb("testhost:22", addr, key); err != nil {
		t.Fatalf("second connection with same key should succeed: %v", err)
	}
}

func TestTOFU_SubsequentConnection_DifferentKey_Rejected(t *testing.T) {
	_ = setupTOFUTest(t)

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := tofuHostKeyCallback()

	if err := cb("testhost:22", addr, key1); err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	if err := cb("testhost:22", addr, key2); err == nil {
		t.Fatal("connection with different key should be rejected")
	}
}

func TestTOFU_UnreadableFile_ReturnsError(t *testing.T) {
	knownHostsPath := setupTOFUTest(t)

	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knownHostsPath, []byte("some data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(knownHostsPath, 0200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(knownHostsPath, 0600) })

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := tofuHostKeyCallback()
	if err := cb("testhost:22", addr, key); err == nil {
		t.Fatal("should return error when known_hosts is not readable")
	}
}
