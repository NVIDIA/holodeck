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
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/NVIDIA/holodeck/internal/logger"
)

type HostKeyPolicy string

const (
	HostKeyPolicyAcceptNew HostKeyPolicy = "accept-new"
	HostKeyPolicyStrict    HostKeyPolicy = "strict"
	HostKeyPolicyOff       HostKeyPolicy = "off"
)

const (
	DefaultMaxAttempts       = 20
	DefaultRetryDelay        = 1 * time.Second
	DefaultHandshakeTimeout  = 15 * time.Second
	DefaultKeepaliveInterval = 30 * time.Second
	DefaultDirectDialTimeout = 10 * time.Second
)

type RetryPolicy struct {
	MaxAttempts int
	Delay       time.Duration
	Exponential bool
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type TimeoutConfig struct {
	Handshake time.Duration
	Keepalive time.Duration
}

type AuthConfig struct {
	User        string
	KeyPath     string
	UseAgent    bool
	AgentSocket string
}

type Dialer struct {
	Auth     AuthConfig
	Retry    RetryPolicy
	Timeouts TimeoutConfig
	HostKey  HostKeyPolicy
	Log      *logger.FunLogger
}

// Dial establishes an *ssh.Client to target over transport t, retrying per the
// RetryPolicy and honoring ctx cancellation between attempts. t == nil selects
// NewDirectTransport(target). On success a self-terminating keepalive goroutine
// is started.
func (d *Dialer) Dial(ctx context.Context, target string, t Transport) (*ssh.Client, error) {
	auth, err := d.authMethods()
	if err != nil {
		return nil, err
	}
	hkcb, err := d.hostKeyCallback(target)
	if err != nil {
		return nil, err
	}
	handshake := d.Timeouts.Handshake
	if handshake <= 0 {
		handshake = DefaultHandshakeTimeout
	}
	cfg := &ssh.ClientConfig{User: d.Auth.User, Auth: auth, HostKeyCallback: hkcb, Timeout: handshake}

	if t == nil {
		t = NewDirectTransport(target)
	}
	addr := hostPort(target)

	maxAttempts := d.Retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		client, derr := d.dialOnce(ctx, addr, t, cfg, handshake)
		if derr == nil {
			startKeepalive(client, d.keepaliveInterval())
			return client, nil
		}
		lastErr = derr
		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d.retryDelay(attempt)):
			}
		}
	}
	return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", target, maxAttempts, lastErr)
}

func (d *Dialer) dialOnce(ctx context.Context, addr string, t Transport, cfg *ssh.ClientConfig, handshake time.Duration) (*ssh.Client, error) {
	conn, err := t.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	// ssh.NewClientConn ignores ClientConfig.Timeout; enforce the handshake
	// bound on the underlying conn (preserves provisioner behavior).
	_ = conn.SetDeadline(time.Now().Add(handshake))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func (d *Dialer) retryDelay(attempt int) time.Duration {
	if !d.Retry.Exponential {
		if d.Retry.Delay > 0 {
			return d.Retry.Delay
		}
		return DefaultRetryDelay
	}
	base := d.Retry.BaseDelay
	if base <= 0 {
		base = DefaultRetryDelay
	}
	delay := base * (1 << attempt)
	if d.Retry.MaxDelay > 0 && delay > d.Retry.MaxDelay {
		delay = d.Retry.MaxDelay
	}
	return delay
}

func (d *Dialer) keepaliveInterval() time.Duration {
	if d.Timeouts.Keepalive > 0 {
		return d.Timeouts.Keepalive
	}
	return DefaultKeepaliveInterval
}

func (d *Dialer) authMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if d.Auth.KeyPath != "" {
		keyPath, err := expandPath(d.Auth.KeyPath)
		if err != nil {
			return nil, err
		}
		key, err := os.ReadFile(keyPath) //nolint:gosec // path from trusted env config
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if d.Auth.UseAgent {
		sock := d.Auth.AgentSocket
		if sock == "" {
			sock = os.Getenv("SSH_AUTH_SOCK")
		}
		if sock == "" {
			return nil, fmt.Errorf("ssh-agent requested but SSH_AUTH_SOCK is unset and AgentSocket is empty")
		}
		conn, err := net.Dial("unix", sock) //nolint:gosec // G704: unix-socket path from trusted SSH_AUTH_SOCK env or operator-set AgentSocket, not attacker-controlled
		if err != nil {
			return nil, fmt.Errorf("connect ssh-agent %s: %w", sock, err)
		}
		methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication material: set Auth.KeyPath or Auth.UseAgent")
	}
	return methods, nil
}

// hostKeyCallback selects verification for the configured policy. T2 replaces
// the accept-new/strict branch with knownhosts-backed verification (making
// strict actually reject unknown hosts).
func (d *Dialer) hostKeyCallback(target string) (ssh.HostKeyCallback, error) {
	if d.HostKey == HostKeyPolicyOff {
		if d.Log != nil {
			d.Log.Warning("SSH host key verification DISABLED (knownHostsPolicy=off) for %s — vulnerable to MITM", target)
		}
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // G106: explicit opt-in via policy=off
	}
	policy := d.HostKey
	if policy == "" {
		policy = HostKeyPolicyAcceptNew
	}
	return HostKeyCallback(policy), nil
}

// hostPort appends ":22" only when target lacks a port.
func hostPort(target string) string {
	if _, _, err := net.SplitHostPort(target); err == nil {
		return target
	}
	return net.JoinHostPort(target, "22")
}

// expandPath expands a leading "~" to the user's home directory.
func expandPath(p string) (string, error) {
	if len(p) == 0 || p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding key path: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

// startKeepalive sends periodic keepalive@holodeck requests; the goroutine
// self-terminates when the client connection closes.
func startKeepalive(client *ssh.Client, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if _, _, err := client.SendRequest("keepalive@holodeck", true, nil); err != nil {
				return
			}
		}
	}()
}
