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
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// BastionTransport tunnels the SSH path to TargetHost through a bastion host.
// Hop 1: Dialer connects to Bastion (its own Auth + HostKey apply). Hop 2: the
// bastion client dials TargetHost:22, yielding a net.Conn the outer Dialer
// wraps for the target handshake. Per-hop known_hosts verification applies.
type BastionTransport struct {
	Bastion    string
	TargetHost string
	Dialer     *Dialer

	hop1 *ssh.Client
}

var _ Transport = (*BastionTransport)(nil)

func (b *BastionTransport) DialContext(ctx context.Context) (net.Conn, error) {
	if b.hop1 == nil {
		client, err := b.Dialer.Dial(ctx, b.Bastion, nil) // hop-1: direct dial to bastion
		if err != nil {
			return nil, fmt.Errorf("bastion hop-1 dial %s: %w", b.Bastion, err)
		}
		b.hop1 = client
	}
	conn, err := b.hop1.DialContext(ctx, "tcp", hostPort(b.TargetHost))
	if err != nil {
		return nil, fmt.Errorf("bastion hop-2 dial %s via %s: %w", b.TargetHost, b.Bastion, err)
	}
	return conn, nil
}

func (b *BastionTransport) Target() string { return b.TargetHost }

func (b *BastionTransport) Close() error {
	if b.hop1 == nil {
		return nil
	}
	err := b.hop1.Close()
	b.hop1 = nil
	return err
}
