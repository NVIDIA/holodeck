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
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"time"
)

// Transport abstracts how SSH connections are established to a target node.
// Each provider controls the transport mechanism (direct TCP, SSM tunnel, etc.)
// while the Provisioner simply receives working connections.
type Transport interface {
	// Dial establishes a TCP connection to the target node's SSH port.
	Dial() (net.Conn, error)
	// Target returns a human-readable identifier for the target (hostname or instance ID).
	Target() string
	// Close releases any resources held by the transport (e.g., SSM tunnel processes).
	Close() error
}

// DirectTransport establishes SSH connections via direct TCP to host:22.
// This is the default transport for single-node environments and the SSH provider.
type DirectTransport struct {
	host string // host:port address (e.g., "10.0.1.5:22")
}

// NewDirectTransport creates a DirectTransport that dials host:22.
func NewDirectTransport(host string) *DirectTransport {
	return &DirectTransport{host: host + ":22"}
}

// Dial connects directly to the host via TCP with a 10-second timeout.
func (d *DirectTransport) Dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", d.host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("direct transport dial %s: %w", d.host, err)
	}
	return conn, nil
}

// Close is a no-op for DirectTransport since there are no resources to release.
func (d *DirectTransport) Close() error {
	return nil
}

// Target returns the host (without port) for display purposes.
func (d *DirectTransport) Target() string {
	host, _, err := net.SplitHostPort(d.host)
	if err != nil {
		return d.host
	}
	return host
}

const (
	// ssmDialMaxRetries is the number of retry attempts when dialing through SSM tunnel.
	ssmDialMaxRetries = 5
	// ssmDialBaseDelay is the base delay for exponential backoff in SSM dial retries.
	ssmDialBaseDelay = 100 * time.Millisecond
	// ssmDialTimeout is the timeout for each individual dial attempt through the SSM tunnel.
	ssmDialTimeout = 500 * time.Millisecond
)

// SSMTransport establishes SSH connections through AWS Systems Manager (SSM)
// port forwarding. This is used for cluster nodes in private subnets that
// do not have public IP addresses.
//
// Known limitation (D2): There is a TOCTOU race between finding a free port
// and starting the SSM session. If the port is taken between these two operations,
// Dial() will fail with "connection refused" after SSM started. The caller should
// retry with a new SSMTransport instance if this occurs.
type SSMTransport struct {
	InstanceID string
	Region     string
	Profile    string

	// cmd holds the running SSM session process so it can be cleaned up.
	cmd       *exec.Cmd
	localPort string
	// stderrBuf captures stderr from the SSM process for diagnostics.
	stderrBuf bytes.Buffer
}

// Dial starts an SSM port-forwarding session and connects to the local tunnel endpoint.
// Uses retry-based dial with exponential backoff (D1) instead of a fixed sleep.
// Idempotent: if a previous session exists, it is closed before starting a new one.
func (s *SSMTransport) Dial() (net.Conn, error) {
	// R1: Close any existing SSM session before starting a new one (idempotency guard)
	if s.cmd != nil {
		_ = s.Close()
	}

	// Find a free local port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("ssm transport: find free port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s.localPort = fmt.Sprintf("%d", port)
	_ = ln.Close() // Release port for SSM to bind (TOCTOU risk accepted per D2)

	// Build and start SSM port-forwarding command
	args := []string{
		"ssm", "start-session",
		"--target", s.InstanceID,
		"--document-name", "AWS-StartPortForwardingSession",
		"--parameters", fmt.Sprintf(`{"portNumber":["22"],"localPortNumber":["%s"]}`, s.localPort),
	}
	if s.Region != "" {
		args = append(args, "--region", s.Region)
	}
	if s.Profile != "" {
		args = append(args, "--profile", s.Profile)
	}

	s.cmd = exec.Command("aws", args...) //nolint:gosec // args are constructed from validated fields
	s.stderrBuf.Reset()
	s.cmd.Stderr = &s.stderrBuf
	if err := s.cmd.Start(); err != nil {
		return nil, fmt.Errorf("ssm transport: start session for %s (stderr: %s): %w", s.InstanceID, s.stderrBuf.String(), err)
	}

	// Retry-based dial with exponential backoff (D1)
	addr := fmt.Sprintf("127.0.0.1:%s", s.localPort)
	conn, err := retryDial(addr, ssmDialMaxRetries, ssmDialBaseDelay)
	if err != nil {
		stderrOutput := s.stderrBuf.String()
		// Clean up the SSM process on dial failure
		_ = s.Close()
		if stderrOutput != "" {
			return nil, fmt.Errorf("ssm transport: dial tunnel for %s (stderr: %s): %w", s.InstanceID, stderrOutput, err)
		}
		return nil, fmt.Errorf("ssm transport: dial tunnel for %s: %w", s.InstanceID, err)
	}

	return conn, nil
}

// Target returns the EC2 instance ID.
func (s *SSMTransport) Target() string {
	return s.InstanceID
}

// Close terminates the SSM port-forwarding session.
func (s *SSMTransport) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			s.cmd = nil
			return fmt.Errorf("ssm transport: kill session: %w", err)
		}
		// Wait to reap the process and avoid zombies
		_ = s.cmd.Wait()
	}
	s.cmd = nil
	return nil
}

// retryDial attempts to connect to addr with exponential backoff.
// Backoff schedule: baseDelay * 2^attempt (e.g., 100ms, 200ms, 400ms, 800ms, 1600ms).
func retryDial(addr string, maxAttempts int, baseDelay time.Duration) (net.Conn, error) {
	var lastErr error
	for attempt := range maxAttempts {
		conn, err := net.DialTimeout("tcp", addr, ssmDialTimeout)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(baseDelay * (1 << attempt))
	}
	return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", addr, maxAttempts, lastErr)
}

// Option is a functional option for configuring a Provisioner.
type Option func(*Provisioner)

// WithTransport sets the transport used for SSH connections.
// If not provided, the Provisioner defaults to DirectTransport(hostUrl).
func WithTransport(t Transport) Option {
	return func(p *Provisioner) {
		p.transport = t
	}
}
