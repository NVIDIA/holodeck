# SSH Subsystem Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse holodeck's three divergent SSH dial implementations plus one shell-out path into one `pkg/sshutil.Dialer` with a `Transport` seam, context-aware retries, `knownhosts`-backed host-key verification with a policy knob, and bastion (two-hop) support — hardening the `ssh.ExitMissingError` mid-provisioning failure class without changing today's provisioner dial envelope.

**Architecture:** `pkg/sshutil` owns the SSH subsystem: a `Dialer` that holds the single dial policy (auth, retry, timeouts, host-key), a `Transport` interface (`DialContext`) with generic `DirectTransport` and `BastionTransport`, and a `knownhosts`-based TOFU layer with cross-process flock. `pkg/provisioner` keeps the AWS-specific `SSMTransport` and adopts the Dialer via a `type Transport = sshutil.Transport` alias plus reuse-with-heal; `cmd/cli` delegates its three call sites to the Dialer with their existing (shorter) retry envelopes expressed as config. Two-tier testing: an in-process `x/crypto/ssh` server for dial mechanics everywhere, and a `real-ssh`-labeled docker `sshd` tier for the full ProviderSSH flow.

**Tech Stack:** Go 1.25, `golang.org/x/crypto/ssh` v0.54.0 (+ vendored subpackages `ssh/knownhosts`, `ssh/agent`), `syscall.Flock`, Ginkgo v2, Docker (real-ssh tier), GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-07-10-ssh-overhaul-design.md`

---

## Global Constraints

Every task inherits these. Reviewers gate on them.

- **Base:** `9ef5a1a1b` (upstream/main + x/crypto v0.54.0 via #847). **Branch:** `feat/851-ssh-overhaul` (already cut; spec doc committed at `7f058e19`).
- **TDD:** tests and implementation land in **separate commits**. RED (failing test, with minimal compile-stubs where Go requires them) precedes GREEN (implementation).
- **Commits:** all signed — `git commit -s -S` — conventional format (`feat|fix|refactor|test|chore(scope): subject`).
- **Lint:** `golangci-lint run --timeout 5m` must be clean against the repo `.golangci.yml` (golangci-lint v2.12.2; `contextcheck`, `gocritic`, `gosec`, `misspell`, `unconvert` enabled). New ctx-taking APIs thread `ctx`; adoption boundaries that must use `context.Background()` carry `//nolint:contextcheck` with a reason (existing `cmd/cli/dryrun/dryrun.go` precedent).
- **Mock suite stays 11/11 green:** `env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u E2E_SSH_KEY make -f tests/Makefile test GINKGO_ARGS="--label-filter=mock"`.
- **No new Go module dependencies.** `ssh/knownhosts` and `ssh/agent` are subpackages of the already-required `golang.org/x/crypto` module — new imports require `go mod vendor` (commit the `vendor/` + `vendor/modules.txt` delta) but **no** new module in `go.mod`.
- **Retry DEFAULTS preserve today's provisioner envelope EXACTLY:** 20 attempts, fixed 1s delay, 15s handshake (via `conn.SetDeadline` around `ssh.NewClientConn`), 30s keepalive. Exponential backoff is **config-opt-in**, never the default.
- **Real-AWS specs never run locally.** The new `real-ssh` tier is credential-free (docker only).

**Acceptance (run before final review):**

```sh
go build ./...
go test ./cmd/cli/... ./pkg/... ./internal/... -count=1
golangci-lint run --timeout 5m
env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u E2E_SSH_KEY \
  make -f tests/Makefile test GINKGO_ARGS="--label-filter=mock"      # 11/11
make -f tests/Makefile test GINKGO_ARGS="--label-filter=real-ssh"    # new tier, local docker
```

---

## Canonical public surface (single source of truth)

Every task MUST match these signatures verbatim. The type-consistency sweep checks them.

```go
// pkg/sshutil
type Transport interface {
    DialContext(ctx context.Context) (net.Conn, error)
    Target() string
    Close() error
}

type HostKeyPolicy string
const (
    HostKeyPolicyAcceptNew HostKeyPolicy = "accept-new" // default; TOFU record-on-first-use
    HostKeyPolicyStrict    HostKeyPolicy = "strict"     // unknown host = error
    HostKeyPolicyOff       HostKeyPolicy = "off"        // insecure, logged loudly
)

type RetryPolicy   struct { MaxAttempts int; Delay time.Duration; Exponential bool; BaseDelay, MaxDelay time.Duration }
type TimeoutConfig struct { Handshake time.Duration; Keepalive time.Duration }
type AuthConfig    struct { User string; KeyPath string; UseAgent bool; AgentSocket string }

type Dialer struct {
    Auth     AuthConfig
    Retry    RetryPolicy
    Timeouts TimeoutConfig
    HostKey  HostKeyPolicy
    Log      *logger.FunLogger
}
func (d *Dialer) Dial(ctx context.Context, target string, t Transport) (*ssh.Client, error)

func NewDirectTransport(host string) *DirectTransport
func HostKeyCallback(policy HostKeyPolicy) ssh.HostKeyCallback // T2
func TOFUHostKeyCallback() ssh.HostKeyCallback                  // back-compat shim → HostKeyCallback(HostKeyPolicyAcceptNew)

type BastionTransport struct { Bastion, TargetHost string; Dialer *Dialer } // T3
```

> **Note on `AuthConfig.User`:** the spec's `Dialer.Dial(ctx, target, t)` signature carries no user parameter; the SSH username lives on `AuthConfig.User` (authentication identity). This resolves a gap in the spec's struct sketch — see the report's ambiguity list.

---

## Task 1: Dialer core + Transport + DirectTransport + in-proc SSH test harness

**Model:** opus. **Deps:** none. This is the deepest task — it defines the seam every other task consumes.

**Files:**
- Create: `pkg/sshutil/transport.go` — `Transport` interface + `DirectTransport`.
- Create: `pkg/sshutil/dialer.go` — `Dialer`, policy types, defaults, keepalive.
- Create: `pkg/sshutil/sshtest/server.go` — exported in-process SSH server harness (see note below).
- Create: `pkg/sshutil/dialer_test.go` — the five contract tests.
- Modify: `vendor/modules.txt` + `vendor/golang.org/x/crypto/ssh/agent/**` via `go mod vendor` (pulls `ssh/agent`).

> **Deviation from the brief's file list (`server_test`):** the harness lives in a small exported package `pkg/sshutil/sshtest` rather than a `server_test.go`, because T3 (bastion), T4 (heal), and T5 (CLI connect) — all of which Dep on T1 — need a live in-proc SSH server; a `_test.go` file cannot be imported across packages. `sshtest` compiles as a normal package but is imported only by `_test.go` files, so it is dead-code-eliminated from the shipped CLI. Flagged in the report.

**Interfaces:**
- Consumes: `golang.org/x/crypto/ssh`, `golang.org/x/crypto/ssh/agent`, `internal/logger`.
- Produces: the entire canonical surface above except `HostKeyCallback`/`BastionTransport` (T2/T3). T1's `Dialer.hostKeyCallback` uses `TOFUHostKeyCallback()` for accept-new/strict and `ssh.InsecureIgnoreHostKey()` for off; T2 refines strict.

### Steps

- [ ] **Step 1 — Write the in-proc SSH harness (`pkg/sshutil/sshtest/server.go`).** This is test infrastructure (not the subject under test); it must be complete so the RED tests can run. Transcribe:

```go
// Package sshtest provides an in-process SSH server for exercising the sshutil
// Dialer against a real handshake, publickey auth, exec channel, keepalive
// accounting, and direct-tcpip forwarding (so it can stand in as a bastion).
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

// Server is a running in-process SSH server. Construct via NewServer.
type Server struct {
	ln         net.Listener
	execOutput string
	forwarding bool
	keepalives atomic.Int32
	forwards   atomic.Int32
}

// Option configures a Server.
type Option func(*Server)

// WithExecOutput sets the stdout the server returns for any exec request.
func WithExecOutput(s string) Option { return func(srv *Server) { srv.execOutput = s } }

// WithForwarding enables direct-tcpip channel forwarding (bastion behavior).
func WithForwarding() Option { return func(srv *Server) { srv.forwarding = true } }

// GenerateKey creates an ed25519 private key, writes it as an OpenSSH PEM file
// under t.TempDir(), and returns the path plus the matching public key.
func GenerateKey(t testing.TB) (keyPath string, pub ssh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath = filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return keyPath, signer.PublicKey()
}

func hostSigner(t testing.TB) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	s, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}
	return s
}

// NewServer starts a server on 127.0.0.1:0 accepting publickey auth for
// clientPub. It is torn down via t.Cleanup.
func NewServer(t testing.TB, clientPub ssh.PublicKey, opts ...Option) *Server {
	t.Helper()
	srv := &Server{}
	for _, o := range opts {
		o(srv)
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(clientPub.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized key")
		},
	}
	cfg.AddHostKey(hostSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv.ln = ln
	go srv.serve(cfg)
	t.Cleanup(func() { _ = ln.Close() })
	return srv
}

// Addr returns the host:port the server is listening on.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Keepalives returns the count of keepalive@holodeck global requests received.
func (s *Server) Keepalives() int { return int(s.keepalives.Load()) }

// Forwards returns the count of direct-tcpip channels opened (bastion hops).
func (s *Server) Forwards() int { return int(s.forwards.Load()) }

func (s *Server) serve(cfg *ssh.ServerConfig) {
	for {
		nConn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(nConn, cfg)
	}
}

func (s *Server) handleConn(nConn net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		_ = nConn.Close()
		return
	}
	go s.handleGlobalRequests(reqs)
	for newCh := range chans {
		switch newCh.ChannelType() {
		case "session":
			ch, chReqs, err := newCh.Accept()
			if err != nil {
				continue
			}
			go s.handleSession(ch, chReqs)
		case "direct-tcpip":
			if s.forwarding {
				go s.handleForward(newCh)
			} else {
				_ = newCh.Reject(ssh.Prohibited, "forwarding disabled")
			}
		default:
			_ = newCh.Reject(ssh.UnknownChannelType, "unsupported channel")
		}
	}
	_ = sc.Close()
}

func (s *Server) handleGlobalRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		if req.Type == "keepalive@holodeck" {
			s.keepalives.Add(1)
		}
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
	}
}

func (s *Server) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "exec", "shell":
			_, _ = io.WriteString(ch, s.execOutput)
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
			_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{0}))
			_ = ch.Close()
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func (s *Server) handleForward(newCh ssh.NewChannel) {
	var p struct {
		DestHost   string
		DestPort   uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(newCh.ExtraData(), &p); err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, "bad direct-tcpip payload")
		return
	}
	dest := net.JoinHostPort(p.DestHost, fmt.Sprintf("%d", p.DestPort))
	upstream, err := net.Dial("tcp", dest)
	if err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	s.forwards.Add(1)
	ch, reqs, err := newCh.Accept()
	if err != nil {
		_ = upstream.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() { _, _ = io.Copy(ch, upstream); _ = ch.Close() }()
	go func() { _, _ = io.Copy(upstream, ch); _ = upstream.Close() }()
}
```

- [ ] **Step 2 — Write the failing Dialer tests (`pkg/sshutil/dialer_test.go`).** Five cases: nil-transport direct dial, retry-count, handshake-timeout, ctx cancellation, keepalive. Transcribe:

```go
package sshutil

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

// countingTransport dials a fixed (unreachable) address, counting attempts.
type countingTransport struct {
	addr  string
	calls atomic.Int32
}

func (c *countingTransport) DialContext(ctx context.Context) (net.Conn, error) {
	c.calls.Add(1)
	d := net.Dialer{Timeout: 100 * time.Millisecond}
	return d.DialContext(ctx, "tcp", c.addr)
}
func (c *countingTransport) Target() string { return c.addr }
func (c *countingTransport) Close() error   { return nil }

func TestDialer_NilTransport_DirectDial(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU known_hosts
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("pong\n"))

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	client, err := d.Dial(context.Background(), srv.Addr(), nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "pong\n", string(out))
}

func TestDialer_RetryCount(t *testing.T) {
	tr := &countingTransport{addr: "127.0.0.1:1"} // nothing listening
	d := &Dialer{
		Auth:  AuthConfig{User: "u", KeyPath: mustKey(t)},
		Retry: RetryPolicy{MaxAttempts: 3, Delay: time.Millisecond},
		Log:   logger.NewLogger(),
	}
	_, err := d.Dial(context.Background(), "unused", tr)
	require.Error(t, err)
	assert.Equal(t, int32(3), tr.calls.Load(), "must attempt exactly MaxAttempts times")
}

func TestDialer_HandshakeTimeout(t *testing.T) {
	// Black-hole listener: accepts TCP, never sends the SSH banner. Without a
	// handshake deadline, ssh.NewClientConn blocks forever and this test hangs.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()

	d := &Dialer{
		Auth:     AuthConfig{User: "u", KeyPath: mustKey(t)},
		HostKey:  HostKeyPolicyOff,
		Retry:    RetryPolicy{MaxAttempts: 2, Delay: time.Millisecond},
		Timeouts: TimeoutConfig{Handshake: 200 * time.Millisecond},
		Log:      logger.NewLogger(),
	}
	start := time.Now()
	_, err = d.Dial(context.Background(), ln.Addr().String(), nil)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second, "handshake deadline must fire, not block")
}

func TestDialer_ContextCancel(t *testing.T) {
	tr := &countingTransport{addr: "127.0.0.1:1"}
	d := &Dialer{
		Auth:  AuthConfig{User: "u", KeyPath: mustKey(t)},
		Retry: RetryPolicy{MaxAttempts: 20, Delay: time.Second}, // long delay
		Log:   logger.NewLogger(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	_, err := d.Dial(ctx, "unused", tr)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), tr.calls.Load(), "cancel during retry delay stops after first attempt")
}

func TestDialer_StartsKeepalive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)

	d := &Dialer{
		Auth:     AuthConfig{User: "u", KeyPath: keyPath},
		HostKey:  HostKeyPolicyAcceptNew,
		Retry:    RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Timeouts: TimeoutConfig{Keepalive: 40 * time.Millisecond},
		Log:      logger.NewLogger(),
	}
	client, err := d.Dial(context.Background(), srv.Addr(), nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	require.Eventually(t, func() bool { return srv.Keepalives() >= 2 },
		2*time.Second, 20*time.Millisecond, "keepalive goroutine must send periodic requests")
}

// mustKey writes a throwaway client key and returns its path.
func mustKey(t *testing.T) string {
	t.Helper()
	p, _ := sshtest.GenerateKey(t)
	return p
}
```

- [ ] **Step 3 — Run the tests; observe RED.** `pkg/sshutil` has no `Dialer`/`DirectTransport` yet, so this fails to compile. Add minimal compile-stubs first (Step 4) so RED is a *behavioral* failure, then commit test+stubs.

- [ ] **Step 4 — Add minimal compile-stubs so the suite compiles and fails behaviorally.** In `pkg/sshutil/transport.go` and `pkg/sshutil/dialer.go`, declare the canonical types and a `Dial`/`NewDirectTransport` that return `errors.New("not implemented")` / a zero transport. Then:

```bash
go test ./pkg/sshutil/... -run TestDialer -count=1
```
Expected: tests FAIL with assertions/`not implemented` (compiles, runs, red). Then commit:
```bash
git -C /Users/eduardoa/src/github/nvidia/holodeck/.worktrees/ssh-overhaul add pkg/sshutil/
git -C /Users/eduardoa/src/github/nvidia/holodeck/.worktrees/ssh-overhaul commit -s -S -m "test(sshutil): Dialer contract + in-proc SSH harness (RED)"
```

- [ ] **Step 5 — Implement `pkg/sshutil/transport.go`.** Replace stubs with:

```go
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
```

- [ ] **Step 6 — Implement `pkg/sshutil/dialer.go`.** Replace stubs with the full Dialer:

```go
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
		conn, err := net.Dial("unix", sock)
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
	return TOFUHostKeyCallback(), nil
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
```

- [ ] **Step 7 — Vendor the `ssh/agent` subpackage.** From the worktree root, run `go mod vendor` (sandbox-disabled if it hits `.git`/cache write denials). Confirm `grep -n 'x/crypto/ssh/agent' vendor/modules.txt` now matches.

- [ ] **Step 8 — Run tests; observe GREEN.**
```bash
go -C /Users/eduardoa/src/github/nvidia/holodeck/.worktrees/ssh-overhaul test ./pkg/sshutil/... -count=1
golangci-lint run --timeout 5m
```
Expected: all `pkg/sshutil` tests pass; lint clean. Commit:
```bash
git -C /Users/eduardoa/src/github/nvidia/holodeck/.worktrees/ssh-overhaul add pkg/sshutil/ vendor/ go.mod go.sum
git -C /Users/eduardoa/src/github/nvidia/holodeck/.worktrees/ssh-overhaul commit -s -S -m "feat(sshutil): SSH Dialer with retry/timeout/keepalive/host-key policy

Introduces pkg/sshutil.Dialer as the single dial policy: ctx-aware retries
(fixed default 20x1s, exponential opt-in), 15s handshake deadline, 30s
keepalive, DirectTransport, and AuthConfig with key-path + ssh-agent. Vendors
golang.org/x/crypto/ssh/agent (no new module)."
```

**Test-quality gate (name the bug each test catches):** retry-count → a broken loop bound (off-by-one / infinite); handshake-timeout → a dropped `SetDeadline` that hangs provisioning; ctx-cancel → an ignored ctx that can't be interrupted; keepalive → a middlebox dropping idle long-running (kubeadm) sessions; direct-dial → the whole auth+handshake path. Delete any test that stays green when its subject is deleted.

---

## Task 2: TOFU → `knownhosts` + cross-process flock + policy knob

**Model:** sonnet. **Deps:** T1. Lands as **three atomic feature slices** (PE condition), each with a RED test commit then a GREEN impl commit: (1) knownhosts reader, (2) flock, (3) policy knob.

**Files:**
- Modify: `pkg/sshutil/tofu.go` — replace the hand-rolled parser with `knownhosts.New`, add `HostKeyCallback(policy)`, add flock.
- Modify: `pkg/sshutil/tofu_test.go` — keep the existing 4 tests green, add the discriminating hashed-entry, typed-`KeyError` mismatch, policy, and flock-contention tests.
- Modify: `pkg/sshutil/dialer.go` — repoint `hostKeyCallback`'s non-off branch to `HostKeyCallback(d.HostKey)`.
- Modify: `vendor/modules.txt` + `vendor/golang.org/x/crypto/ssh/knownhosts/**` via `go mod vendor`.

**Interfaces:**
- Consumes: `golang.org/x/crypto/ssh/knownhosts`, `syscall`, T1's `HostKeyPolicy` constants.
- Produces:
  - `func HostKeyCallback(policy HostKeyPolicy) ssh.HostKeyCallback`
  - `func TOFUHostKeyCallback() ssh.HostKeyCallback` (now `= HostKeyCallback(HostKeyPolicyAcceptNew)`)

### Slice 1 — knownhosts reader

- [ ] **Step 1 — Write the discriminating hashed-entry guard test (RED).** This is the anti-theater core: it FAILS against the old `parts[0] == hostname` parser and PASSES against `knownhosts`. Append to `tofu_test.go`:

```go
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
```
Add imports `knownhosts "golang.org/x/crypto/ssh/knownhosts"`, testify `assert`/`require`. Run `go mod vendor` first (so `knownhosts` resolves), then:
```bash
go -C .../.worktrees/ssh-overhaul test ./pkg/sshutil/ -run TestTOFU_HashedEntry -count=1
```
Expected: FAIL — `HostKeyCallback` does not exist yet (add a stub returning the old TOFU callback so it compiles, then the assertion fails because the old parser accepts key2). Commit test + stub + vendor:
```bash
git -C .../.worktrees/ssh-overhaul add pkg/sshutil/ vendor/ go.mod go.sum
git -C .../.worktrees/ssh-overhaul commit -s -S -m "test(sshutil): knownhosts hashed-entry mismatch guard (RED)"
```

- [ ] **Step 2 — Implement the knownhosts reader (GREEN).** Rewrite `tofu.go`:

```go
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
```
Repoint `dialer.go`'s `hostKeyCallback` non-off branch:
```go
	policy := d.HostKey
	if policy == "" {
		policy = HostKeyPolicyAcceptNew
	}
	return HostKeyCallback(policy), nil
```
Run — the four existing TOFU tests + the hashed-entry guard pass:
```bash
go -C .../.worktrees/ssh-overhaul test ./pkg/sshutil/ -count=1
```
Commit:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(sshutil): verify host keys via x/crypto knownhosts"
```

### Slice 2 — cross-process flock

- [ ] **Step 3 — Write the subprocess flock-contention test (RED).** A helper subprocess (re-exec of the test binary) grabs `LOCK_EX` and holds it; the parent's `LOCK_NB` attempt must fail with `EWOULDBLOCK`, proving the verify path locks cross-process. Append to `tofu_test.go`:

```go
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
```
Run to see RED (`verifyOrRecord` does not yet flock, but this test locks directly — RED is that the *production* path lacks flock; to make the RED meaningful, first assert on a helper `withKnownHostsLock`; see Step 4). Simplest honest sequence: introduce the test now proving the OS-level guarantee, then Step 4 wires `syscall.Flock` into `HostKeyCallback` and asserts no corruption under concurrency. Commit the test:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "test(sshutil): cross-process flock contention on known_hosts (RED)"
```

- [ ] **Step 4 — Add flock to the verify path (GREEN).** Wrap `verifyOrRecord` in `HostKeyCallback` with an advisory lock:

```go
	lockF, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600) //nolint:gosec // path from UserCacheDir
	if err != nil {
		return fmt.Errorf("open known_hosts for lock: %w", err)
	}
	defer func() { _ = lockF.Close() }()
	if err := syscall.Flock(int(lockF.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock known_hosts: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lockF.Fd()), syscall.LOCK_UN) }()
	return verifyOrRecord(path, policy, hostname, remote, key)
```
Add a concurrency test asserting N goroutines recording the same host produce exactly one line (no interleaved corruption). Run `go test -race ./pkg/sshutil/ -count=1`; commit:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(sshutil): flock known_hosts read-modify-write"
```

### Slice 3 — policy knob

- [ ] **Step 5 — Write policy tests (RED).** `strict` rejects an unknown host; `off` accepts any key and writes nothing; `accept-new` records then matches. Append three focused tests (`TestTOFU_Strict_UnknownRejected`, `TestTOFU_Off_AcceptsAndSkipsFile`, `TestTOFU_AcceptNew_RecordsThenMatches`). The `strict` and `off` branches already exist from Slices 1–2, so these guard against regressions of the policy semantics; if strict is broken to fall through to record, `TestTOFU_Strict_UnknownRejected` goes red. Run to confirm they pass now (they exercise already-implemented branches) — if any is green with the branch deleted, it is theater; make `strict` genuinely reject. Commit tests:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "test(sshutil): host-key policy accept-new/strict/off"
```

- [ ] **Step 6 — Confirm GREEN + lint, finalize slice.** `go test ./pkg/sshutil/ -count=1 && golangci-lint run --timeout 5m`. If the policy branches needed any tightening, that impl delta commits as:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(sshutil): host-key policy knob (accept-new default, strict, off)"
```

**Test-quality gate:** hashed-entry guard catches the legacy parser silently accepting a MITM key when the stored entry is hashed (the exact `tofu.go` weakness); flock test catches concurrent multi-node provisioning corrupting known_hosts; strict test catches an unknown host being silently trusted when the operator asked for strict.

---

## Task 3: BastionTransport (two-hop)

**Model:** sonnet. **Deps:** T1. Uses `Client.DialContext` (verified present at `vendor/golang.org/x/crypto/ssh/tcpip.go:379`).

**Files:**
- Create: `pkg/sshutil/bastion.go` — `BastionTransport`.
- Create: `pkg/sshutil/bastion_test.go` — two-server chain test (target = `sshtest.NewServer`, bastion = `sshtest.NewServer(WithForwarding())`).

**Interfaces:**
- Consumes: T1's `Dialer`, `Transport`, `hostPort`, and `sshtest.NewServer`/`WithForwarding`/`GenerateKey`.
- Produces:
```go
type BastionTransport struct {
    Bastion    string  // bastion host (host or host:port)
    TargetHost string  // final destination host (host or host:port)
    Dialer     *Dialer // connects hop-1 to the bastion; its Auth/HostKey apply to the bastion
}
func (b *BastionTransport) DialContext(ctx context.Context) (net.Conn, error)
func (b *BastionTransport) Target() string
func (b *BastionTransport) Close() error
```

### Steps

- [ ] **Step 1 — Write the failing two-hop test (RED).** Transcribe `bastion_test.go`:

```go
package sshutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestBastionTransport_TwoHop(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate per-hop TOFU
	keyPath, pub := sshtest.GenerateKey(t)
	target := sshtest.NewServer(t, pub, sshtest.WithExecOutput("hello-from-target\n"))
	bastion := sshtest.NewServer(t, pub, sshtest.WithForwarding())

	d := &Dialer{
		Auth:    AuthConfig{User: "tester", KeyPath: keyPath},
		HostKey: HostKeyPolicyAcceptNew,
		Retry:   RetryPolicy{MaxAttempts: 3, Delay: 10 * time.Millisecond},
		Log:     logger.NewLogger(),
	}
	bt := &BastionTransport{Bastion: bastion.Addr(), TargetHost: target.Addr(), Dialer: d}
	defer func() { _ = bt.Close() }()

	client, err := d.Dial(context.Background(), target.Addr(), bt)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Prove the chain reaches the TARGET: scripted exec output round-trips.
	sess, err := client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "hello-from-target\n", string(out))

	// Prove the BASTION forwarded the hop, and Close() releases hop-1 cleanly.
	assert.GreaterOrEqual(t, bastion.Forwards(), 1, "bastion must forward the direct-tcpip hop")
	assert.NoError(t, bt.Close())
}
```
Add a compile-stub `BastionTransport` (methods return `nil, errors.New("not implemented")`). Run → RED. Commit:
```bash
git -C .../.worktrees/ssh-overhaul add pkg/sshutil/
git -C .../.worktrees/ssh-overhaul commit -s -S -m "test(sshutil): bastion two-hop transport (RED)"
```

- [ ] **Step 2 — Implement `bastion.go` (GREEN).**

```go
package sshutil

import (
	"context"
	"fmt"

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
```
(Add `"net"` to the import block.) Run → GREEN; lint. Commit:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(sshutil): BastionTransport two-hop dialer"
```

**Test-quality gate:** the exec output proves hop-2 actually reaches the target (not the bastion); `Forwards() >= 1` proves the traffic went *through* the bastion, not around it. If `DialContext` mistakenly returned a direct conn to the target, `Forwards()` would be 0 → red.

---

## Task 4: Provisioner adoption — Transport alias, SSM ctx, reuse-with-heal

**Model:** opus. **Deps:** T1, T6 (SSHConfig mapping). **Run-after note:** the optional bastion-wiring line references `sshutil.BastionTransport` (T3); if T3 has not merged when T4 runs, omit that one branch and add it in a follow-up commit — the non-bastion envelope is the required deliverable. Flagged in the report.

**Files:**
- Modify: `pkg/provisioner/transport.go` — `type Transport = sshutil.Transport`; alias `DirectTransport`; `SSMTransport.Dial` → `DialContext(ctx)` with `exec.CommandContext`; keep `retryDial`.
- Modify: `pkg/provisioner/provisioner.go` — `Dialer` field + `ensureClient`; delete `connectOrDie`/`startKeepalive`/`resetConnection`→heal; `WithSSHConfig` option; `sshHandshakeTimeout` etc. constants removed (defaults now in sshutil).
- Modify: `pkg/provisioner/transport_test.go` + `pkg/provisioner/ssh_config_test.go` — rename `Dial()` → `DialContext(ctx)` in `countingTransport`/`DirectTransport` call sites (minimal edits).
- Create: `pkg/provisioner/heal_test.go` — reuse-with-heal test using `sshtest.NewServer`.

**Interfaces:**
- Consumes: `sshutil.Dialer`, `sshutil.NewDirectTransport`, `sshutil.Transport`, `v1alpha1.SSHConfig` (T6).
- Produces (provisioner package):
```go
type Transport = sshutil.Transport
type DirectTransport = sshutil.DirectTransport
func NewDirectTransport(host string) *sshutil.DirectTransport { return sshutil.NewDirectTransport(host) }
func WithSSHConfig(cfg *v1alpha1.SSHConfig) Option
func (p *Provisioner) ensureClient(ctx context.Context) error
func (s *SSMTransport) DialContext(ctx context.Context) (net.Conn, error) // renamed from Dial()
```

### Steps

- [ ] **Step 1 — Write the heal test (RED) `pkg/provisioner/heal_test.go`.** Proves: `New` connects to a live server; a dropped client is transparently re-dialed by `ensureClient`; a live client is reused. Transcribe:

```go
package provisioner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestProvisioner_EnsureClient_HealsDroppedConnection(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate TOFU
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("ok\n"))

	p, err := New(logger.NewLogger(), keyPath, "tester", srv.Addr())
	require.NoError(t, err)
	require.NotNil(t, p.Client)

	// Simulate a mid-provisioning drop.
	require.NoError(t, p.Client.Close())

	// ensureClient must heal (re-dial) transparently.
	require.NoError(t, p.ensureClient(context.Background()))
	sess, err := p.Client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(out))

	// A live client is reused (same pointer) on the next ensureClient.
	before := p.Client
	require.NoError(t, p.ensureClient(context.Background()))
	assert.Same(t, before, p.Client, "live connection must be reused, not re-dialed")
}
```
Run → RED (`ensureClient` undefined; `New` still uses `connectOrDie`). Note: `New(log, keyPath, userName, hostUrl)` already accepts `srv.Addr()` (host:port) once T1's `NewDirectTransport` handles the port. Commit:
```bash
git -C .../.worktrees/ssh-overhaul add pkg/provisioner/heal_test.go
git -C .../.worktrees/ssh-overhaul commit -s -S -m "test(provisioner): reuse-with-heal via in-proc SSH server (RED)"
```

- [ ] **Step 2 — Alias the Transport + ctx-ify SSMTransport (`transport.go`).**
  - Replace the local `Transport` interface + `DirectTransport` struct/methods with:
    ```go
    type Transport = sshutil.Transport
    type DirectTransport = sshutil.DirectTransport
    func NewDirectTransport(host string) *sshutil.DirectTransport { return sshutil.NewDirectTransport(host) }
    ```
  - Rename `SSMTransport.Dial()` → `DialContext(ctx context.Context) (net.Conn, error)`; replace `exec.Command("aws", args...)` with `exec.CommandContext(ctx, "aws", args...)`; `retryDial` keeps its own per-attempt `net.DialTimeout` (SSM-local) — thread `ctx` into it via `(&net.Dialer{Timeout: ssmDialTimeout}).DialContext(ctx, "tcp", addr)` (mechanical).
  - Keep the `//nolint:gosec` on the aws command.

- [ ] **Step 3 — Adopt the Dialer in `provisioner.go`.**
  - Add fields: `dialer *sshutil.Dialer`, `sshConfig *v1alpha1.SSHConfig` to `Provisioner`.
  - `New`: after applying options and defaulting the transport, build the dialer then heal-connect:
    ```go
    if p.dialer == nil {
        p.dialer = dialerFromSSHConfig(keyPath, userName, p.sshConfig, log)
    }
    if err := p.ensureClient(context.Background()); err != nil { //nolint:contextcheck // Run() has no ctx param (follow-up); Background is the boundary
        return nil, fmt.Errorf("failed to connect to %s: %w", hostUrl, err)
    }
    ```
  - Add `ensureClient` (probe + heal):
    ```go
    func (p *Provisioner) ensureClient(ctx context.Context) error {
        if p.Client != nil {
            if sess, err := p.Client.NewSession(); err == nil {
                _ = sess.Close()
                return nil
            }
            _ = p.Client.Close()
            p.Client = nil
        }
        client, err := p.dialer.Dial(ctx, p.HostUrl, p.transport)
        if err != nil {
            return fmt.Errorf("failed to connect to %s: %w", p.HostUrl, err)
        }
        p.Client = client
        return nil
    }
    ```
  - Add the SSHConfig→Dialer mapping (consumes T6; `cfg == nil` yields the exact historical envelope — MaxRetries/timeouts left zero so sshutil applies its 20×1s/15s/30s defaults):
    ```go
    func dialerFromSSHConfig(keyPath, userName string, cfg *v1alpha1.SSHConfig, log *logger.FunLogger) *sshutil.Dialer {
        d := &sshutil.Dialer{
            Auth:    sshutil.AuthConfig{User: userName, KeyPath: keyPath},
            HostKey: sshutil.HostKeyPolicyAcceptNew,
            Log:     log,
        }
        if cfg == nil {
            return d
        }
        if cfg.KnownHostsPolicy != "" {
            d.HostKey = sshutil.HostKeyPolicy(cfg.KnownHostsPolicy)
        }
        d.Auth.UseAgent = cfg.UseAgent
        d.Auth.AgentSocket = cfg.AgentSocket
        if cfg.MaxRetries > 0 {
            d.Retry.MaxAttempts = cfg.MaxRetries
        }
        d.Timeouts.Handshake = cfg.HandshakeTimeout.Duration
        d.Timeouts.Keepalive = cfg.KeepaliveInterval.Duration
        return d
    }
    ```
    (`ConnectTimeout` maps to `DirectTransport`'s dial timeout; wiring that through is mechanical and may be deferred — note it in the report if omitted.)
  - `provision()`: replace the top `if p.Client != nil { close }; connectOrDie(...)` block with `if err := p.ensureClient(context.Background()); err != nil { return err }` (carry `//nolint:contextcheck`).
  - `resetConnection()`: keep it as the **force-refresh** used between dependencies so the docker-group-membership change takes effect — it closes + nils `p.Client`, and the *next* `ensureClient` re-dials. Do NOT make it reuse; that is the one place a fresh connection is mandatory. Add a one-line comment stating this invariant.
  - `waitForNodeReboot()`: keep the 10×30s outer loop; replace its inner `connectOrDie` with `ensureClient` (it nils `p.Client` first, so ensureClient re-dials).
  - Delete `connectOrDie`, `startKeepalive`, and the `sshMaxRetries`/`sshRetryDelay`/`sshKeepaliveInterval`/`sshHandshakeTimeout` constants (now owned by sshutil).

- [ ] **Step 4 — Update the two existing provisioner test files (minimal renames).**
  - `ssh_config_test.go`: `countingTransport.Dial()` → `DialContext(ctx context.Context)` (ignore ctx or thread it into the inner `net.DialTimeout`→`Dialer.DialContext`); `TestNew_HandshakeTimeout` stays — `New` still enforces the 15s handshake via the Dialer default envelope, so ≥2 black-hole attempts still complete within 45s.
  - `transport_test.go`: `TestDirectTransport_Dial_*` call `dt.DialContext(context.Background())`; `countingTransport.Dial` (if present) → `DialContext`; SSM `retryDial`/`Target`/`Close` tests unchanged; `TestDirectTransport_ImplementsTransport`/`TestNodeInfo_TransportField` compile unchanged via the alias.

- [ ] **Step 5 — Run everything; observe GREEN.**
```bash
go -C .../.worktrees/ssh-overhaul build ./...
go -C .../.worktrees/ssh-overhaul test ./pkg/provisioner/... ./pkg/sshutil/... -count=1
golangci-lint run --timeout 5m
env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u E2E_SSH_KEY \
  make -f tests/Makefile test GINKGO_ARGS="--label-filter=mock"   # 11/11 unchanged
```
Commit the implementation:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "refactor(provisioner): adopt sshutil.Dialer, alias Transport, SSM ctx

connectOrDie/startKeepalive collapse into sshutil.Dialer; Transport becomes a
type alias for sshutil.Transport; SSMTransport gains ctx via
exec.CommandContext; New/provision/waitForNodeReboot heal a dropped client via
a NewSession liveness probe. resetConnection remains the deliberate
force-refresh between dependencies (docker group membership). Default dial
envelope (20x1s, 15s handshake, 30s keepalive) is unchanged."
```

**Test-quality gate:** the heal test catches a dropped connection that today's `provision()` re-dial masks only by luck; the `assert.Same` reuse assertion catches a regression that re-dials every phase (wasteful, and would defeat the point of reuse). The mock suite (11/11) is the integration guard that the alias + SSM ctx did not break the AWS lifecycle.

---

## Task 5: CLI adoption — common / dryrun / ssh delegate to the Dialer

**Model:** sonnet. **Deps:** T1, T6.

**Files:**
- Modify: `cmd/cli/common/host.go` — `ConnectSSH` builds a `sshutil.Dialer` with the CLI's 3×2s / 30s-handshake envelope (via explicit `RetryPolicy`, NOT the 20×1s default); signature unchanged.
- Modify: `cmd/cli/dryrun/dryrun.go` — `connectOrDie` delegates to a `sshutil.Dialer`, gaining a 15s handshake timeout (fixes the current *no-timeout* bug).
- Modify: `cmd/cli/ssh/ssh.go` — command-exec path already routes through `common.ConnectSSH`; no logic change (interactive system-ssh path unchanged). Add nothing unless a compile fix is needed.
- Create: `cmd/cli/common/host_test.go` — connect happy-path + bounded-failure test (in-proc server).
- Create: `cmd/cli/dryrun/dryrun_test.go` — the handshake-timeout-configured guard.

**Interfaces:**
- Consumes: `sshutil.Dialer`, `sshutil.AuthConfig`, `sshutil.RetryPolicy`, `sshutil.TimeoutConfig`, `sshtest`.
- Produces (unchanged public signatures):
```go
func ConnectSSH(log *logger.FunLogger, keyPath, userName, hostUrl string) (*ssh.Client, error)
```

### Steps

- [ ] **Step 1 — Write `common/host_test.go` (RED).** Happy path against an in-proc server + bounded failure against a dead port (proves the 3-attempt envelope doesn't hang):

```go
package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestConnectSSH_Connects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub, sshtest.WithExecOutput("hi\n"))

	client, err := ConnectSSH(logger.NewLogger(), keyPath, "tester", srv.Addr())
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	sess, err := client.NewSession()
	require.NoError(t, err)
	out, err := sess.Output("noop")
	require.NoError(t, err)
	assert.Equal(t, "hi\n", string(out))
}

func TestConnectSSH_FailsBoundedOnDeadPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, _ := sshtest.GenerateKey(t)
	start := time.Now()
	_, err := ConnectSSH(logger.NewLogger(), keyPath, "tester", "127.0.0.1:1")
	require.Error(t, err)
	assert.Less(t, time.Since(start), 15*time.Second, "3x2s envelope must not hang")
}
```
Run → RED (still uses `ssh.Dial` directly; happy path may pass but is not yet routed through the Dialer — keep the test; it guards the behavior post-refactor). Commit tests.

- [ ] **Step 2 — Refactor `ConnectSSH` (GREEN).** Replace the body with:
```go
func ConnectSSH(log *logger.FunLogger, keyPath, userName, hostUrl string) (*ssh.Client, error) {
	d := &sshutil.Dialer{
		Auth:     sshutil.AuthConfig{User: userName, KeyPath: keyPath},
		HostKey:  sshutil.HostKeyPolicyAcceptNew,
		Retry:    sshutil.RetryPolicy{MaxAttempts: 3, Delay: 2 * time.Second},
		Timeouts: sshutil.TimeoutConfig{Handshake: 30 * time.Second},
		Log:      log,
	}
	return d.Dial(context.Background(), hostUrl, nil) //nolint:contextcheck // CLI action boundary; no ctx to thread yet
}
```
Delete the now-unused `sshMaxRetries`/`sshRetryDelay` consts and the inline key parsing (the Dialer owns it). Keep `GetHostURL` untouched. Run → GREEN; commit impl.

- [ ] **Step 3 — Write `dryrun/dryrun_test.go` (RED) — the handshake-timeout guard.** The current `connectOrDie` sets NO handshake timeout; the fix routes it through a Dialer with a 15s handshake. Guard the exact fix by asserting the constructed dialer carries a non-zero handshake, plus a happy-path connect:

```go
package dryrun

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/sshutil/sshtest"
)

func TestDryrunDialer_HasHandshakeTimeout(t *testing.T) {
	d := dryrunDialer("/tmp/none", "u", logger.NewLogger())
	assert.NotZero(t, d.Timeouts.Handshake, "dryrun must set a handshake timeout (was missing)")
	assert.Equal(t, 20, d.Retry.MaxAttempts, "dryrun keeps its 20-attempt envelope")
}

func TestDryrunConnect_Succeeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyPath, pub := sshtest.GenerateKey(t)
	srv := sshtest.NewServer(t, pub)
	require.NoError(t, connectOrDie(keyPath, "tester", srv.Addr()))
}
```
Run → RED (`dryrunDialer` undefined). Commit test.

- [ ] **Step 4 — Refactor dryrun `connectOrDie` (GREEN).** Extract the dialer so it is testable and gains the handshake timeout:
```go
func dryrunDialer(keyPath, userName string, log *logger.FunLogger) *sshutil.Dialer {
	return &sshutil.Dialer{
		Auth:     sshutil.AuthConfig{User: userName, KeyPath: keyPath},
		HostKey:  sshutil.HostKeyPolicyAcceptNew,
		Retry:    sshutil.RetryPolicy{MaxAttempts: 20, Delay: 1 * time.Second},
		Timeouts: sshutil.TimeoutConfig{Handshake: 15 * time.Second},
		Log:      log,
	}
}

func connectOrDie(keyPath, userName, hostUrl string) error {
	client, err := dryrunDialer(keyPath, userName, m.log /* or a package logger */).Dial(context.Background(), hostUrl, nil) //nolint:contextcheck
	if err != nil {
		return err
	}
	_ = client.Close()
	return nil
}
```
Note: `connectOrDie` is a free function today; pass the logger in (thread `m.log` from `run`, or make it a method `func (m command) connectOrDie(...)`). Keep the change minimal and mechanical. Run → GREEN; `go build ./cmd/...`; `golangci-lint run --timeout 5m`. Commit impl:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "refactor(cli): route common/dryrun/ssh through sshutil.Dialer

ConnectSSH keeps its 3x2s/30s envelope via explicit RetryPolicy; dryrun gains
the 15s handshake timeout it was missing. Interactive ssh (system binary) and
the ssh exec path (via ConnectSSH) are unchanged."
```

**Test-quality gate:** `TestDryrunDialer_HasHandshakeTimeout` catches the exact reported bug (dryrun blocking forever on an unresponsive host); `TestConnectSSH_FailsBoundedOnDeadPort` catches a regression that swaps in the 20×1s default and stalls the CLI.

---

## Task 6: `auth.sshConfig` API types + validation + jyaml round-trip

**Model:** sonnet. **Deps:** T1 (nominal — for policy-value consistency; T6 does NOT import `pkg/sshutil`, to avoid coupling the CRD types to the dial package).

**Files:**
- Modify: `api/holodeck/v1alpha1/types.go` — add `SSHConfig` (+ `BastionConfig`) to `Auth`.
- Create: `api/holodeck/v1alpha1/sshconfig_test.go` — jyaml round-trip + `Validate()` tests.

**Interfaces:**
- Consumes: `metav1.Duration` (already imported) for `"30s"`-style YAML durations that round-trip through jyaml (YAML→JSON→struct).
- Produces:
```go
type SSHConfig struct {
    Bastion           *BastionConfig  `json:"bastion,omitempty"`
    UseAgent          bool            `json:"useAgent,omitempty"`
    AgentSocket       string          `json:"agentSocket,omitempty"`
    KnownHostsPolicy  string          `json:"knownHostsPolicy,omitempty"` // accept-new|strict|off
    ConnectTimeout    metav1.Duration `json:"connectTimeout,omitempty"`
    HandshakeTimeout  metav1.Duration `json:"handshakeTimeout,omitempty"`
    KeepaliveInterval metav1.Duration `json:"keepaliveInterval,omitempty"`
    MaxRetries        int             `json:"maxRetries,omitempty"`
}
type BastionConfig struct {
    Host       string `json:"host"`
    Username   string `json:"username,omitempty"`
    PrivateKey string `json:"privateKey,omitempty"`
}
func (c *SSHConfig) Validate() error
```

### Steps

- [ ] **Step 1 — Write `sshconfig_test.go` (RED).** Round-trip a fully-populated config through `jyaml.MarshalYAML` → `jyaml.Unmarshal[SSHConfig]` and assert deep-equality; plus valid/invalid `Validate()` cases. Transcribe:

```go
package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/pkg/jyaml"
)

func TestSSHConfig_RoundTrip(t *testing.T) {
	in := SSHConfig{
		Bastion:           &BastionConfig{Host: "bastion.example.com", Username: "ec2-user", PrivateKey: "/keys/bastion.pem"},
		UseAgent:          true,
		AgentSocket:       "/tmp/agent.sock",
		KnownHostsPolicy:  "strict",
		ConnectTimeout:    metav1.Duration{Duration: 30 * time.Second},
		HandshakeTimeout:  metav1.Duration{Duration: 15 * time.Second},
		KeepaliveInterval: metav1.Duration{Duration: 30 * time.Second},
		MaxRetries:        20,
	}
	y, err := jyaml.MarshalYAML(in)
	require.NoError(t, err)
	out, err := jyaml.Unmarshal[SSHConfig](y)
	require.NoError(t, err)
	assert.Equal(t, in, out, "sshConfig must survive a YAML round-trip")
}

func TestSSHConfig_Validate(t *testing.T) {
	require.NoError(t, (&SSHConfig{KnownHostsPolicy: "accept-new"}).Validate())
	require.NoError(t, (&SSHConfig{}).Validate()) // all-optional
	assert.Error(t, (&SSHConfig{KnownHostsPolicy: "yolo"}).Validate())
	assert.Error(t, (&SSHConfig{MaxRetries: -1}).Validate())
	assert.Error(t, (&SSHConfig{Bastion: &BastionConfig{Host: ""}}).Validate())
}
```
Run → RED (types undefined). Commit test + minimal type stubs so it compiles.

- [ ] **Step 2 — Add the types + `Validate` (GREEN).** In `types.go`, add the `SSHConfig`/`BastionConfig` structs above, wire `SSHConfig *SSHConfig \`json:"sshConfig,omitempty"\`` into `Auth` (with `// +optional`), and:
```go
func (c *SSHConfig) Validate() error {
	switch c.KnownHostsPolicy {
	case "", "accept-new", "strict", "off":
	default:
		return fmt.Errorf("invalid knownHostsPolicy %q (want accept-new|strict|off)", c.KnownHostsPolicy)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("maxRetries must be >= 0, got %d", c.MaxRetries)
	}
	if c.Bastion != nil && c.Bastion.Host == "" {
		return fmt.Errorf("bastion.host is required when bastion is set")
	}
	return nil
}
```
Add the kubebuilder enum marker on `KnownHostsPolicy` (`// +kubebuilder:validation:Enum=accept-new;strict;off`). Add `"fmt"` import if absent. If `zz_generated.deepcopy.go` is regenerated by the repo's codegen, run it and include the delta; otherwise hand-add `DeepCopyInto` for the new pointer fields (mechanical) — **verify** `Auth`/`EnvironmentSpec` deepcopy still compiles.
Run → GREEN:
```bash
go -C .../.worktrees/ssh-overhaul test ./api/... -count=1
golangci-lint run --timeout 5m
```
Commit:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(api): auth.sshConfig (bastion/agent/policy/timeouts)"
```

**Test-quality gate:** the round-trip catches a wrong/missing json tag (a field silently dropped on load — the #1 CRD bug); `Validate` catches an operator typo in `knownHostsPolicy` reaching the Dialer as an unknown policy. The `metav1.Duration` fields specifically guard `"30s"` parsing (a plain `time.Duration` would fail to unmarshal the string form).

---

## Task 7: `real-ssh` E2E — docker sshd harness + label + CI job

**Model:** opus. **Deps:** T4, T5.

**Files:**
- Create: `tests/e2e_ssh_test.go` — `Label("real-ssh")` spec driving the full ProviderSSH flow against a docker `sshd`.
- Create: `tests/data/test_ssh.yaml` — a minimal `provider: ssh` env (all installs off; one inline custom template that writes an observable marker).
- Modify: `.github/workflows/e2e-smoke.yaml` — add a credential-free `e2e-real-ssh` job.

**Interfaces:**
- Consumes: `os/exec` (docker), `provisioner.New`/`Run`, `cmd/cli/common.ConnectSSH` (or the `holodeck ssh` exec path), `sshtest.GenerateKey`-style in-test keygen (or `ssh-keygen`), Ginkgo v2.

### Docker image decision (verify before relying on it)

The spec names `linuxserver/openssh-server`. That image is **Alpine-based and ships no `bash`**, but holodeck's provision `Shebang` is `#! /usr/bin/env bash`. The harness therefore MUST make `bash` available. Two acceptable routes — pick one during implementation and **verify the tag is pullable** (`docker pull` + `docker image inspect`):
- **(A, recommended)** `docker exec <id> apk add --no-cache bash` after the container is up, keeping `linuxserver/openssh-server` per the spec. Pin by digest.
- **(B)** Use an Ubuntu-based sshd image that already has bash. Pin by digest.

Flag this in the report as a spec/reality gap (Alpine vs bash shebang). The plan below uses route A.

### Steps

- [ ] **Step 1 — Write `tests/data/test_ssh.yaml`.** A minimal env that does no heavy installs and drops a marker file via a custom template:
```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: ssh-e2e
spec:
  provider: ssh
  auth:
    keyName: e2e
    username: holo            # overridden at runtime to the container user
    privateKey: /tmp/replaced-at-runtime
    publicKey: /tmp/replaced-at-runtime.pub
  instance:
    hostUrl: 127.0.0.1        # overridden at runtime (host:port set via env/transport)
  nvidiaDriver: {install: false}
  containerRuntime: {install: false}
  nvidiaContainerToolkit: {install: false}
  kubernetes: {install: false}
  customTemplates:
    - name: marker
      phase: post-install
      inline: |
        echo "holodeck-was-here" > /tmp/holodeck-marker
```
`hostUrl` nests under `instance:` because `EnvironmentSpec` embeds `Instance` with `json:"instance"` (Go promotes `Spec.HostUrl`, but JSON/YAML nests it). Verify the fixture loads via a `jyaml.UnmarshalFromFile[v1alpha1.Environment]` assertion in the test before relying on it.

- [ ] **Step 2 — Write the `real-ssh` spec (RED) `tests/e2e_ssh_test.go`.** Hard-fail (not Skip) when docker is unavailable and the `real-ssh` label is selected (QA condition). Structure:

```go
package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func requireDocker() {
	GinkgoHelper()
	if _, err := exec.LookPath("docker"); err != nil {
		Fail("real-ssh tier requires docker, but 'docker' is not on PATH")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		Fail(fmt.Sprintf("real-ssh tier requires a running docker daemon: %v\n%s", err, out))
	}
}

var _ = Describe("Real SSH ProviderSSH E2E", Label("real-ssh"), func() {
	It("provisions over SSH and reads back the on-container marker", func() {
		requireDocker()

		// 1. Generate a keypair; write private + public to temp files.
		// 2. docker run -d --rm -p 127.0.0.1:0:2222 -e USER_NAME=holo
		//    -e PUBLIC_KEY="$(cat pub)" <linuxserver/openssh-server@sha256:...>
		// 3. docker exec <id> apk add --no-cache bash   (route A)
		// 4. hostPort := `docker port <id> 2222` -> 127.0.0.1:<port>
		// 5. Load tests/data/test_ssh.yaml; set PrivateKey, Username="holo",
		//    HostUrl/host to 127.0.0.1 and the SSH port via a DirectTransport
		//    (or set the env hostUrl+port). provisioner.New(...) -> p.Run(env).
		// 6. Read the marker back through the holodeck ssh exec path:
		//    client := common.ConnectSSH(...); session.Output("cat /tmp/holodeck-marker")
		// 7. Assert output == "holodeck-was-here\n".
		// DeferCleanup: docker kill <id>.

		Skip("implement the docker orchestration described above") // remove in Step 3
	})
})
```
Run with the label to confirm the spec is discovered and (with the `Skip` still present) the suite is green-but-pending; then remove the `Skip` and watch it RED against a real run. Commit the RED spec + fixture:
```bash
git -C .../.worktrees/ssh-overhaul add tests/e2e_ssh_test.go tests/data/test_ssh.yaml
git -C .../.worktrees/ssh-overhaul commit -s -S -m "test(e2e): real-ssh docker sshd harness (RED)"
```

- [ ] **Step 3 — Implement the docker orchestration (GREEN).** Fill in steps 1–7: keygen (reuse the `crypto/ed25519` + `ssh.MarshalPrivateKey` pattern, write `id` + `id.pub` via `ssh.MarshalAuthorizedKey`), container lifecycle via `exec.Command("docker", ...)` (`//nolint:gosec` — controlled test args), port discovery via `docker port`, a readiness wait (poll TCP + one SSH handshake), then the full `provisioner.New` → `Run` → `common.ConnectSSH` + `session.Output` read-back. Assert the marker. `DeferCleanup` kills the container. Run locally:
```bash
make -f tests/Makefile test GINKGO_ARGS="--label-filter=real-ssh"
```
Expected: 1 spec passes; marker asserted. Commit impl + CI job (Step 4) together.

- [ ] **Step 4 — Add the credential-free CI job to `.github/workflows/e2e-smoke.yaml`.** Append a job that needs no AWS secrets (GitHub runners provide docker):
```yaml
  e2e-real-ssh:
    runs-on: linux-amd64-cpu4
    name: E2E Real SSH
    steps:
    - name: Checkout code
      uses: actions/checkout@v7
    - name: Install Go
      uses: actions/setup-go@v6
      with:
        go-version: 'stable'
        check-latest: true
    - name: Install dependencies
      run: |
        sudo apt-get update
        sudo apt-get install -y make
    - name: Run real-ssh e2e test
      env:
        LOG_ARTIFACT_DIR: e2e_logs
      run: |
        make -f tests/Makefile test GINKGO_ARGS="--label-filter='real-ssh' --json-report ginkgo.json"
    - name: Archive Ginkgo logs
      if: always()
      uses: actions/upload-artifact@v7
      with:
        name: ginkgo-real-ssh-logs
        path: ginkgo.json
        retention-days: 15
```
Verify the mock job's `--label-filter` still selects only `mock` (real-ssh must NOT leak into the mock lane). Run the acceptance block from Global Constraints. Commit:
```bash
git -C .../.worktrees/ssh-overhaul commit -s -S -am "feat(e2e): real-ssh ProviderSSH E2E + CI job

Drives env.yaml -> provision -> holodeck ssh exec against a docker openssh
container, asserting an on-container marker written by the provision script.
docker-availability hard-fails (never Skip) when the real-ssh label is
selected. New credential-free e2e-real-ssh job in the PR lane."
```

**Test-quality gate:** the marker read-back proves the FULL path (dial → provision script executes on the container → exec reads the side-effect), not just a handshake; the hard-fail-on-no-docker prevents the tier silently passing as a no-op (the exact QA condition). Per the learned anti-pattern, this E2E is the ONLY signal that the seam works cross-process — green unit tests do not certify it.

---

## Execution

Subagent-driven-development, chief dispatch loop. The chief (this session's operator) owns the branch `feat/851-ssh-overhaul`, dispatches one task at a time to a routed builder, gates each with an adversarial critic, and runs the final whole-branch review.

**Task briefs:** materialize one brief per task at `.superpowers/sdd/task-N-brief.md`, each carrying: the task section above verbatim, a consumer census from pasted `grep -rln` output, the Global Constraints, the report-file path + status vocabulary (DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT), and the closing line **"LAST ACTION: SendMessage to main with the deliverable."**

**Dependency order (respect the table):** T1 → then T2, T3, T6 in parallel (all Dep only T1) → T4 (Dep T1, T6; run after T3 for the bastion-wiring line) and T5 (Dep T1, T6) → T7 (Dep T4, T5).

**Model routing (set `model:` explicitly on every dispatch):**

| Task | Builder model | Review gate |
|---|---|---|
| T1 Dialer core + harness | opus | opus |
| T2 knownhosts + flock + policy | sonnet | opus |
| T3 bastion | sonnet | opus |
| T4 provisioner adoption | opus | opus |
| T5 CLI adoption | sonnet | opus |
| T6 sshConfig types | sonnet | opus |
| T7 real-ssh E2E | opus | opus |

**Final whole-branch review:** fable — adversarial, spec-coverage sweep against `docs/superpowers/specs/2026-07-10-ssh-overhaul-design.md`, the full acceptance block, and a type-consistency sweep (`Dialer.Dial`, `Transport.DialContext`, `HostKeyPolicy*` identical across all files).

**Merge gate:** every task's RED/GREEN evidence present; `golangci-lint run --timeout 5m` clean; mock suite 11/11; `real-ssh` green on local docker; no `context.Background()` inside library functions except the annotated CLI/provisioner boundaries.

