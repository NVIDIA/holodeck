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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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

	cb := TOFUHostKeyCallback()
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

	cb := TOFUHostKeyCallback()

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

	cb := TOFUHostKeyCallback()

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

	cb := TOFUHostKeyCallback()
	if err := cb("testhost:22", addr, key); err == nil {
		t.Fatal("should return error when known_hosts is not readable")
	}
}

func TestTOFU_HashedEntry_MismatchRejected(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)
	host := "testhost:22"

	// Seed a HASHED known_hosts entry for host→key1 (the |1|salt|hash form
	// ssh-keygen -H produces). The legacy parser splits on space, sees
	// parts[0]=="|1|...", never matches "testhost:22", and treats the host as
	// unknown — so it would RECORD key2 and accept. knownhosts matches the
	// hashed line and rejects the changed key.
	hashed := knownhosts.HashHostname(knownhosts.Normalize(host))
	line := knownhosts.Line([]string{hashed}, key1)
	require.NoError(t, os.WriteFile(path, []byte(line+"\n"), 0600))

	cb := HostKeyCallback(HostKeyPolicyAcceptNew)
	err := cb(host, &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}, key2)
	require.Error(t, err, "changed key against a hashed entry must be rejected")

	var keyErr *knownhosts.KeyError
	require.ErrorAs(t, err, &keyErr)
	assert.NotEmpty(t, keyErr.Want, "KeyError.Want must name the recorded (hashed) key")
	assert.Contains(t, err.Error(), "host key mismatch")
}

func TestTOFU_Flock_CrossProcessExclusion(t *testing.T) {
	if os.Getenv("HOLODECK_FLOCK_CHILD") == "1" {
		// Child: lock the file exclusively, signal readiness, hold until killed.
		path := os.Getenv("HOLODECK_FLOCK_PATH")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			os.Exit(2)
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
			os.Exit(3)
		}
		fmt.Println("locked")
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	require.NoError(t, os.WriteFile(path, nil, 0600))

	cmd := exec.Command(os.Args[0], "-test.run", "TestTOFU_Flock_CrossProcessExclusion") //nolint:gosec // re-exec of the test binary
	cmd.Env = append(os.Environ(), "HOLODECK_FLOCK_CHILD=1", "HOLODECK_FLOCK_PATH="+path)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	buf := make([]byte, 6)
	_, _ = io.ReadFull(stdout, buf) // wait for "locked"

	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	assert.ErrorIs(t, err, syscall.EWOULDBLOCK, "parent must be blocked while child holds LOCK_EX")
}
