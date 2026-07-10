# SSH Subsystem Overhaul — Design Spec

Date: 2026-07-10
Status: approved by Carlos (design gate, this date); milestone 1 of the multi-cloud plan
Base: upstream/main @ 9ef5a1a1b (includes golang.org/x/crypto v0.54.0 via #847)
Branch: feat/ssh-overhaul

## Problem

Holodeck has three independent, divergently-parameterized SSH dial
implementations and one shell-out path:

| Site | Retries | Delay | Handshake timeout | Transport-aware | Keepalive |
|---|---|---|---|---|---|
| `pkg/provisioner/provisioner.go connectOrDie` | 20 | 1s fixed | 15s (manual `SetDeadline`) | yes | yes |
| `cmd/cli/common/host.go ConnectSSH` | 3 | 2s fixed | 30s (`ClientConfig.Timeout`) | no | no |
| `cmd/cli/dryrun/dryrun.go connectOrDie` | 20 | 1s fixed | **none** | no | no |
| `cmd/cli/ssh/ssh.go` (interactive) | system `ssh` binary | — | — | no | — |

Host-key verification is a hand-rolled TOFU (`pkg/sshutil/tofu.go`) with a
custom-but-coincidentally-OpenSSH-compatible file format, process-local
locking only, naive `parts[0] == hostname` parsing (silently fails on hashed
entries, no key-type awareness), and the interactive path bypasses it
entirely (system ssh + `StrictHostKeyChecking=accept-new` pointed at the same
file). No context.Context anywhere in the dial path. No bastion support. No
ssh-agent support. SSH parameters are threaded as loose positional strings.

Today's real-AWS CI failure (`ssh.ExitMissingError` mid-provisioning, run
29093625575) is exactly the failure class this milestone hardens against.

## Decisions (all panel-reviewed, all user-approved 2026-07-10)

1. **Seam**: `pkg/sshutil` owns the SSH subsystem — `Dialer` + `Transport`
   interface + generic transports. Panel: SOFT-DISSENT (DA preferred
   conn-factory; PE, QA held); Carlos confirmed the recommendation.
2. **TOFU**: adopt `golang.org/x/crypto/ssh/knownhosts` (same vendored
   module, no new dependency) + cross-process flock + policy knob. Panel:
   SOFT-DISSENT (DA preferred custom-format+flock; PE, QA held); Carlos
   confirmed.
3. **Testing**: two-tier — in-process x/crypto SSH server for dialer
   mechanics; `real-ssh` labeled docker sshd tier for the full ProviderSSH
   flow. Panel: SOFT-DISSENT (DA preferred container-only; PE, QA held);
   Carlos confirmed.
4. **ctx scope** (decided, not paneled — Go convention): all new API is
   ctx-first; adoption sites pass their existing context or
   `context.Background()`; full ctx threading through `provisioner.Run` is a
   follow-up, not this milestone (YAGNI).

## Design

### pkg/sshutil (new surface)

```go
// Transport abstracts how the TCP path to the SSH port is established.
type Transport interface {
    DialContext(ctx context.Context) (net.Conn, error)
    Target() string // human-readable target id
    Close() error   // releases held resources (tunnels, hop clients)
}

type DirectTransport struct{ ... }   // moves from pkg/provisioner; net.Dialer w/ ctx
type BastionTransport struct{ ... }  // NEW: hop1 = Dialer to bastion; hop2 = hop1client.Dial("tcp", target:22)

// Dialer owns the single dial policy for the repo.
type Dialer struct {
    Auth      AuthConfig    // key path; optional ssh-agent (socket or SSH_AUTH_SOCK)
    Retry     RetryPolicy   // MaxAttempts, Delay; optional exponential (BaseDelay/MaxDelay)
    Timeouts  TimeoutConfig // Handshake, Keepalive interval
    HostKey   HostKeyPolicy // accept-new (default) | strict | off
    Log       *logger.FunLogger
}

func (d *Dialer) Dial(ctx context.Context, target string, t Transport) (*ssh.Client, error)
```

- Defaults preserve today's provisioner envelope EXACTLY: 20 attempts, fixed
  1s delay, 15s handshake (enforced via `conn.SetDeadline` around
  `ssh.NewClientConn`, as today), keepalive 30s. Exponential backoff is
  config-opt-in, not default — the failure-path latency envelope must not
  silently change (fixed 20×1s ≈ 20s of sleeps vs capped-exponential ≈ 2min).
- `t == nil` ⇒ DirectTransport(target). Dryrun's missing handshake timeout is
  fixed by construction.
- Keepalive goroutine keeps current self-terminating design; Dialer owns
  start.

### pkg/provisioner

- `type Transport = sshutil.Transport` (alias — exported `NodeInfo.Transport`
  keeps compiling; compile-time assertions updated in place).
- `SSMTransport` stays here (AWS-specific, shells to `aws ssm`); gains ctx via
  `exec.CommandContext`; implements `sshutil.Transport`.
- `connectOrDie` becomes a thin `Dialer.Dial` call. **Reuse-with-heal**: the
  Provisioner keeps one `*ssh.Client`; before each phase it liveness-probes
  (cheap `NewSession` ping) and transparently re-dials on failure.
  `waitForNodeReboot`'s outer 10×30s loop survives unchanged in semantics
  (reboots are expected mid-provisioning); `resetConnection` collapses into
  the heal path.

### cmd/cli

- `common.ConnectSSH`, dryrun `connectOrDie`, `ssh` command-exec path all
  delegate to `sshutil.Dialer` (their current retry counts become SSHConfig
  defaults where behavior must not regress: CLI paths keep shorter profiles
  via config, not code forks).
- Interactive `ssh` keeps the system-ssh UX but the divergence narrows: both
  paths now share genuine known_hosts semantics (see TOFU).

### TOFU / known_hosts

- Verify via `knownhosts.New` (handles hashed entries, multiple keys,
  key-type awareness, typed `*knownhosts.KeyError`).
- Writes remain unhashed OpenSSH lines (current file format is already valid
  — migration is read-side only).
- Cross-process `flock` (stdlib `syscall.Flock`, LOCK_EX) around
  read-modify-write; process-local mutex retained.
- `HostKeyPolicy`: `accept-new` (TOFU, default — current behavior),
  `strict` (unknown host = error), `off` (insecure, logged loudly).
- Lands as separate atomic commits (PE condition).

### api/holodeck/v1alpha1

```yaml
auth:
  keyName: ...
  privateKey: ...
  sshConfig:            # NEW, all optional (back-compat)
    bastion: {host, username, privateKey}
    useAgent: bool      # or agentSocket: path
    knownHostsPolicy: accept-new|strict|off
    connectTimeout: 30s
    handshakeTimeout: 15s
    keepaliveInterval: 30s
    maxRetries: 20
```

Types + validation + jyaml round-trip tests live in the types task; wiring
into Dialer construction lands with the adoption tasks (T4/T5), fixing the
kickoff plan's ownership overlap.

### Testing (two-tier)

- **Tier 1 (everywhere, `go test`)**: in-process x/crypto/ssh server (real
  handshake/auth/channels, scripted exec). Specs: retry/backoff timing,
  handshake timeout, TOFU record + mismatch (assert typed KeyError and exact
  message), hashed-entry fixture generated via `ssh-keygen -H`
  (discriminating guard — fails on the old parser), bastion two-hop, keepalive,
  flock contention via subprocess LOCK_NB. Never labeled E2E; validates
  protocol mechanics, not on-VM behavior (honesty constraint).
- **Tier 2 (`real-ssh` label)**: openssh-server container via `os/exec docker
  run` (zero new Go deps; SSMTransport precedent). Drives the FULL
  ProviderSSH flow: env.yaml → provision → `holodeck ssh -- cmd`, asserting
  observable on-container side-effects. CI: credential-free job in the PR
  lane (e2e-smoke.yaml); **docker-availability check hard-fails the job —
  never silently skips** (QA condition). Local: `make -f tests/Makefile test
  GINKGO_ARGS="--label-filter=real-ssh"`.
- Mock suite (label=mock, 11 specs) untouched and must stay green.

## Out of scope

- Full context threading through `provisioner.Run` (follow-up).
- `pkg/provider/aws` / `internal/aws` public API changes.
- Multiplexing across *processes* (ControlMaster-style); reuse is
  per-process.
- EKS/GCE/GKE work (separate milestones; this seam is their prerequisite).

## Task decomposition

| # | Task | Owns | Deps | Model |
|---|---|---|---|---|
| T1 | Dialer core + Transport + Direct + in-proc test server | pkg/sshutil/{dialer,transport,server_test}* | — | opus |
| T2 | TOFU → knownhosts + flock + policy | pkg/sshutil/tofu* | T1 | sonnet |
| T3 | BastionTransport (two-hop) | pkg/sshutil/bastion* | T1 | sonnet |
| T4 | Provisioner adoption: alias, SSM ctx, reuse-with-heal | pkg/provisioner/* | T1,T6 | opus |
| T5 | CLI adoption: common/dryrun/ssh delegate to Dialer | cmd/cli/* | T1,T6 | sonnet |
| T6 | SSHConfig types + validation + jyaml tests | api/holodeck/v1alpha1/* | T1 | sonnet |
| T7 | real-ssh E2E harness + label + CI job | tests/*, .github/workflows/e2e-smoke.yaml | T4,T5 | opus |

All review gates: opus. Final whole-branch review: fable. TDD throughout
(tests before implementation; test and impl in separate commits). Every brief
carries: consumer census from pasted `grep -rln` output, golangci-lint
v2.12.2 gate, report file + status vocabulary, "LAST ACTION: SendMessage".

## Verification (acceptance)

```sh
go build ./...
go test ./cmd/cli/... ./pkg/... ./internal/... -count=1
golangci-lint run --timeout 5m          # repo .golangci.yml, CI pin v2.12.2
env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u E2E_SSH_KEY \
  make -f tests/Makefile test GINKGO_ARGS="--label-filter=mock"      # 11/11+
make -f tests/Makefile test GINKGO_ARGS="--label-filter=real-ssh"   # new tier, local docker
```

Session goal acceptance (Stop-hook enforced): `golangci-lint run`,
`go test ./pkg/sshutil/... -v`, `go test ./pkg/provisioner/... -v`.
