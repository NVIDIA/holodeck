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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var tofuMu sync.Mutex

// TOFUHostKeyCallback preserves the historical accept-new behavior.
func TOFUHostKeyCallback() ssh.HostKeyCallback { return HostKeyCallback(HostKeyPolicyAcceptNew) }

// HostKeyCallback verifies host keys against $CACHE/holodeck/known_hosts using
// x/crypto/ssh/knownhosts (hashed entries, multiple keys, key-type awareness).
// accept-new records unknown hosts (TOFU); strict rejects them; off skips.
func HostKeyCallback(policy HostKeyPolicy) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if policy == HostKeyPolicyOff {
			return nil
		}
		path, err := knownHostsPath()
		if err != nil {
			return err
		}
		tofuMu.Lock()
		defer tofuMu.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return fmt.Errorf("create known_hosts dir: %w", err)
		}
		// Slice 2 wraps this read-verify-append in a cross-process flock.
		return verifyOrRecord(path, policy, hostname, remote, key)
	}
}

func verifyOrRecord(path string, policy HostKeyPolicy, hostname string, remote net.Addr, key ssh.PublicKey) error {
	// knownhosts.New requires the file to exist; create it empty on first use.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			return fmt.Errorf("init known_hosts: %w", err)
		}
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return fmt.Errorf("load known_hosts: %w", err)
	}
	verr := cb(hostname, remote, key)
	if verr == nil {
		return nil // known and matches
	}
	var keyErr *knownhosts.KeyError
	if errors.As(verr, &keyErr) {
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("host key mismatch for %s (possible MITM): %w", hostname, verr)
		}
		// Unknown host (Want empty).
		if policy == HostKeyPolicyStrict {
			return fmt.Errorf("unknown host %s rejected by strict host-key policy: %w", hostname, verr)
		}
		return appendKnownHost(path, hostname, key)
	}
	return verr
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) //nolint:gosec // path from UserCacheDir
	if err != nil {
		return fmt.Errorf("open known_hosts for append: %w", err)
	}
	defer func() { _ = f.Close() }()
	// knownhosts.Line emits a valid, unhashed OpenSSH line (format unchanged).
	if _, err := fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)); err != nil {
		return fmt.Errorf("write known host: %w", err)
	}
	return nil
}

func knownHostsPath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine cache directory for TOFU host keys: %w", err)
	}
	return filepath.Join(base, "holodeck", "known_hosts"), nil
}
