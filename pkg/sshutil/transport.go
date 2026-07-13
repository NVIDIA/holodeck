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
	"time"
)

// Transport abstracts how the TCP path to the SSH port is established.
type Transport interface {
	DialContext(ctx context.Context) (net.Conn, error)
	Target() string
	Close() error
}

// DirectTransport dials host:22 directly over TCP (net.Dialer, ctx-aware).
type DirectTransport struct {
	host string
	// timeout bounds the TCP dial; 0 selects DefaultDirectDialTimeout so an
	// unset auth.sshConfig.connectTimeout preserves the legacy 10s behavior.
	timeout time.Duration
}

// NewDirectTransport dials host; ":22" is appended only when host has no port.
// The TCP dial timeout is DefaultDirectDialTimeout; use NewDirectTransportTimeout
// to override it from auth.sshConfig.connectTimeout.
func NewDirectTransport(host string) *DirectTransport {
	addr := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		addr = net.JoinHostPort(host, "22")
	}
	return &DirectTransport{host: addr}
}

// NewDirectTransportTimeout is NewDirectTransport with an explicit TCP dial
// timeout. A timeout <= 0 keeps DefaultDirectDialTimeout, so an unset
// connectTimeout preserves today's dial envelope exactly.
func NewDirectTransportTimeout(host string, timeout time.Duration) *DirectTransport {
	d := NewDirectTransport(host)
	if timeout > 0 {
		d.timeout = timeout
	}
	return d
}

func (d *DirectTransport) DialContext(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: d.DialTimeout()}
	conn, err := dialer.DialContext(ctx, "tcp", d.host)
	if err != nil {
		return nil, fmt.Errorf("direct transport dial %s: %w", d.host, err)
	}
	return conn, nil
}

// DialTimeout reports the effective TCP dial timeout, resolving an unset
// (0) timeout to DefaultDirectDialTimeout.
func (d *DirectTransport) DialTimeout() time.Duration {
	if d.timeout <= 0 {
		return DefaultDirectDialTimeout
	}
	return d.timeout
}

func (d *DirectTransport) Close() error { return nil }

func (d *DirectTransport) Target() string {
	host, _, err := net.SplitHostPort(d.host)
	if err != nil {
		return d.host
	}
	return host
}
