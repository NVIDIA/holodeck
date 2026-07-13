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
	"strings"
	"sync"
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

// hashKnownHostsEntry seeds a plaintext known_hosts line for host->key and
// hashes it via the real ssh-keygen -H binary (an implementation
// independent of golang.org/x/crypto/ssh/knownhosts, the package under
// test), returning the resulting hashed line. This is deliberately NOT
// knownhosts.HashHostname: that helper lives in the same package as the
// parser being exercised, so a shared hashing bug could make the guard
// pass for the wrong reason. ssh-keygen -H is the canonical, independently
// implemented producer of the OpenSSH |1|salt|hash format.
func hashKnownHostsEntry(t *testing.T, host string, key ssh.PublicKey) string {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available in this environment")
	}

	dir := t.TempDir()
	seedPath := filepath.Join(dir, "known_hosts")
	plain := knownhosts.Normalize(host) + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	require.NoError(t, os.WriteFile(seedPath, []byte(plain+"\n"), 0600))

	out, err := exec.Command("ssh-keygen", "-H", "-f", seedPath).CombinedOutput() //nolint:gosec // fixed binary name, test-controlled path
	require.NoError(t, err, "ssh-keygen -H failed: %s", out)

	hashed, err := os.ReadFile(seedPath) //nolint:gosec // path from t.TempDir()
	require.NoError(t, err)
	line := strings.TrimSpace(string(hashed))
	require.True(t, strings.HasPrefix(line, "|1|"), "ssh-keygen -H must produce a |1|salt|hash entry, got: %s", line)
	return line
}

func TestTOFU_HashedEntry_MismatchRejected(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	key1 := generateTestKey(t)
	key2 := generateTestKey(t)
	host := "testhost:22"

	// Seed a HASHED known_hosts entry for host→key1, produced by the real
	// ssh-keygen -H binary (see hashKnownHostsEntry). The legacy parser
	// splits on space, sees parts[0]=="|1|...", never matches "testhost:22",
	// and treats the host as unknown — so it would RECORD key2 and accept.
	// knownhosts matches the hashed line and rejects the changed key.
	line := hashKnownHostsEntry(t, host, key1)
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
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600) //nolint:gosec // path from test-controlled env var
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

	f, err := os.OpenFile(path, os.O_RDWR, 0600) //nolint:gosec // path from t.TempDir()
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	assert.ErrorIs(t, err, syscall.EWOULDBLOCK, "parent must be blocked while child holds LOCK_EX")
}

// TestTOFU_Flock_BlocksConcurrentHostKeyCallback is the discriminating guard
// for the production wiring: TestTOFU_Flock_CrossProcessExclusion above only
// proves the OS primitive works via raw syscalls, never calling production
// code, so it cannot fail if HostKeyCallback never acquires the lock. Here an
// independent open-file-description on the same known_hosts path holds
// LOCK_EX directly (BSD/Linux flock exclusion applies per open-file
// description, not per-process, so this reproduces cross-process contention
// deterministically without a subprocess). Before the lock is wired into
// HostKeyCallback, the callback races straight through and the goroutine
// finishes almost instantly — this assertion fails. After Step 4 wires the
// lock, the callback blocks until the external holder releases it.
func TestTOFU_Flock_BlocksConcurrentHostKeyCallback(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, nil, 0600))

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	holder, err := os.OpenFile(path, os.O_RDWR, 0600) //nolint:gosec // path from setupTOFUTest's tmpdir
	require.NoError(t, err)
	defer func() { _ = holder.Close() }()
	require.NoError(t, syscall.Flock(int(holder.Fd()), syscall.LOCK_EX))

	done := make(chan error, 1)
	go func() {
		cb := HostKeyCallback(HostKeyPolicyAcceptNew)
		done <- cb("testhost:22", addr, key)
	}()

	select {
	case <-done:
		t.Fatal("HostKeyCallback returned while the known_hosts lock was held externally — it did not block on flock")
	case <-time.After(200 * time.Millisecond):
		// Expected: HostKeyCallback is still blocked waiting for the lock.
	}

	require.NoError(t, syscall.Flock(int(holder.Fd()), syscall.LOCK_UN))

	select {
	case err := <-done:
		assert.NoError(t, err, "HostKeyCallback should succeed once the external lock is released")
	case <-time.After(2 * time.Second):
		t.Fatal("HostKeyCallback did not complete after the external lock was released")
	}
}

// TestTOFU_ConcurrentRecord_NoCorruption documents the read-verify-append
// path's behavior under concurrency, per the brief's Step 4 requirement: N
// goroutines recording the same unknown host must produce exactly one
// known_hosts line, never an interleaved/corrupted write. Note this specific
// property already holds pre-flock (tofuMu already serializes the whole
// HostKeyCallback body within one process) — the flock discriminator is
// TestTOFU_Flock_BlocksConcurrentHostKeyCallback above, which targets
// cross-process contention that tofuMu cannot cover.
func TestTOFU_ConcurrentRecord_NoCorruption(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cb := HostKeyCallback(HostKeyPolicyAcceptNew)
			errs[i] = cb("testhost:22", addr, key)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoErrorf(t, err, "goroutine %d", i)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test helper with controlled tmpdir path
	require.NoError(t, err)
	lines := 0
	for _, l := range splitNonEmptyLines(string(data)) {
		_ = l
		lines++
	}
	assert.Equal(t, 1, lines, "concurrent recording of the same host must produce exactly one known_hosts line")

	cb := HostKeyCallback(HostKeyPolicyAcceptNew)
	assert.NoError(t, cb("testhost:22", addr, key), "the recorded entry must verify cleanly afterward")
}

func splitNonEmptyLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// TestTOFU_Strict_UnknownRejected guards the strict policy branch added in
// Slice 1: an unrecorded host must be rejected, not silently trusted. If
// strict is broken to fall through to record (the pre-T2 TOFU behavior), this
// test goes red.
func TestTOFU_Strict_UnknownRejected(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := HostKeyCallback(HostKeyPolicyStrict)
	err := cb("testhost:22", addr, key)
	require.Error(t, err, "strict must reject an unknown host")
	assert.Contains(t, err.Error(), "strict host-key policy")

	data, rerr := os.ReadFile(path) //nolint:gosec // test helper with controlled tmpdir path
	require.NoError(t, rerr)
	assert.Empty(t, splitNonEmptyLines(string(data)), "strict must not record the rejected host")
}

// TestTOFU_Off_AcceptsAndSkipsFile guards the off policy branch: any key is
// accepted and the known_hosts file is never touched (no directory, no
// write). If off ever fell through to the accept-new record path, this test
// goes red on the file-existence assertion.
func TestTOFU_Off_AcceptsAndSkipsFile(t *testing.T) {
	path := setupTOFUTest(t)

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := HostKeyCallback(HostKeyPolicyOff)
	require.NoError(t, cb("testhost:22", addr, key))

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "off must not create known_hosts at all")
}

// TestTOFU_AcceptNew_RecordsThenMatches guards the accept-new (TOFU) branch
// via the new HostKeyCallback(policy) entry point: first contact records,
// second contact with the same key matches, without requiring strict
// pre-approval. If accept-new stopped recording, the second call would see
// an unknown host again and this test would go red on the "already known"
// assertion below.
func TestTOFU_AcceptNew_RecordsThenMatches(t *testing.T) {
	path := setupTOFUTest(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))

	key := generateTestKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 22}

	cb := HostKeyCallback(HostKeyPolicyAcceptNew)
	require.NoError(t, cb("testhost:22", addr, key), "first contact should record")

	data, err := os.ReadFile(path) //nolint:gosec // test helper with controlled tmpdir path
	require.NoError(t, err)
	require.Len(t, splitNonEmptyLines(string(data)), 1, "first contact must write exactly one entry")

	require.NoError(t, cb("testhost:22", addr, key), "second contact with the same key should match, not re-record")

	data2, err := os.ReadFile(path) //nolint:gosec // test helper with controlled tmpdir path
	require.NoError(t, err)
	assert.Len(t, splitNonEmptyLines(string(data2)), 1, "matching a known host must not append a duplicate entry")
}
