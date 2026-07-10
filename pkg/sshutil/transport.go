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
)

// Transport abstracts how the TCP path to the SSH port is established.
type Transport interface {
	DialContext(ctx context.Context) (net.Conn, error)
	Target() string
	Close() error
}

// DirectTransport dials host:22 directly over TCP (net.Dialer, ctx-aware).
type DirectTransport struct{ host string }

// NewDirectTransport dials host; ":22" is appended only when host has no port.
func NewDirectTransport(host string) *DirectTransport {
	addr := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		addr = net.JoinHostPort(host, "22")
	}
	return &DirectTransport{host: addr}
}

func (d *DirectTransport) DialContext(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: DefaultDirectDialTimeout} // const from dialer.go
	conn, err := dialer.DialContext(ctx, "tcp", d.host)
	if err != nil {
		return nil, fmt.Errorf("direct transport dial %s: %w", d.host, err)
	}
	return conn, nil
}

func (d *DirectTransport) Close() error { return nil }

func (d *DirectTransport) Target() string {
	host, _, err := net.SplitHostPort(d.host)
	if err != nil {
		return d.host
	}
	return host
}
