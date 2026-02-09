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

	"golang.org/x/crypto/ssh"
)

// TOFUHostKeyCallback returns an ssh.HostKeyCallback implementing
// Trust-On-First-Use host key verification.
func TOFUHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		cacheBase, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("cannot determine cache directory for TOFU host keys: %w", err)
		}
		knownHostsPath := filepath.Join(cacheBase, "holodeck", "known_hosts")

		if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
			return fmt.Errorf("failed to create known_hosts directory: %w", err)
		}

		keyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
		host := hostname

		// Try to read existing known hosts file
		data, err := os.ReadFile(knownHostsPath) // nolint:gosec // path from UserCacheDir + static components
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read known_hosts: %w", err)
		}
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) == 2 && parts[0] == host {
					if strings.TrimSpace(parts[1]) == keyStr {
						return nil // Key matches
					}
					return fmt.Errorf("host key mismatch for %s: stored key differs from presented key (possible MITM)", host)
				}
			}
		}

		// First connection to this host: record the key (TOFU)
		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nolint:gosec
		if err != nil {
			return fmt.Errorf("failed to open known_hosts for writing: %w", err)
		}
		defer func() { _ = f.Close() }()

		if _, err := fmt.Fprintf(f, "%s %s\n", host, keyStr); err != nil {
			return fmt.Errorf("failed to write known host: %w", err)
		}

		return nil
	}
}
