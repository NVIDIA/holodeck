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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// tofuMu serialises access to the known_hosts file so that concurrent SSH
// connections (e.g. during multi-node cluster provisioning) do not race on the
// read-then-write.
var tofuMu sync.Mutex

// TOFUHostKeyCallback returns an ssh.HostKeyCallback implementing a
// Trust-On-First-Use (TOFU) pattern for SSH host key verification. On first
// connection to a host, the key is recorded in a holodeck-specific known_hosts
// file at $CACHE/holodeck/known_hosts (where $CACHE is os.UserCacheDir). On
// subsequent connections the stored key is compared and a mismatch — indicating
// a potential MITM attack — is rejected with an error.
func TOFUHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		tofuMu.Lock()
		defer tofuMu.Unlock()

		cacheBase, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("cannot determine cache directory for TOFU host keys: %w", err)
		}
		knownHostsPath := filepath.Join(cacheBase, "holodeck", "known_hosts")

		if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
			return fmt.Errorf("failed to create known_hosts directory: %w", err)
		}

		keyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

		// Try to read existing known hosts file
		data, err := os.ReadFile(knownHostsPath) //nolint:gosec // path from UserCacheDir + static components
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read known_hosts: %w", err)
		}
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) == 2 && parts[0] == hostname {
					if strings.TrimSpace(parts[1]) == keyStr {
						return nil // Key matches
					}
					return fmt.Errorf("host key mismatch for %s: stored key differs from presented key (possible MITM)", hostname)
				}
			}
		}

		// First connection to this host: record the key (TOFU)
		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) //nolint:gosec
		if err != nil {
			return fmt.Errorf("failed to open known_hosts for writing: %w", err)
		}
		defer func() { _ = f.Close() }()

		if _, err := fmt.Fprintf(f, "%s %s\n", hostname, keyStr); err != nil {
			return fmt.Errorf("failed to write known host: %w", err)
		}

		return nil
	}
}
