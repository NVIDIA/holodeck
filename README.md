# Holodeck

[![Latest Release](https://img.shields.io/github/v/release/NVIDIA/holodeck?label=latest%20release)](https://github.com/NVIDIA/holodeck/releases/latest)

[![CI Pipeline](https://github.com/NVIDIA/holodeck/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/NVIDIA/holodeck/actions/workflows/ci.yaml)

A tool for creating and managing GPU-ready Cloud test environments.

---

## 📖 Documentation

- [Quick Start](docs/quick-start.md)
- [Prerequisites](docs/prerequisites.md)
- [Commands Reference](docs/commands/)
- [Contributing Guide](docs/contributing/)
- [Examples](docs/examples/)
- [Latest Release](https://github.com/NVIDIA/holodeck/releases/latest)

---

## ✨ Features

- **Multi-OS Support**: Ubuntu, Rocky Linux 9, Amazon Linux 2023 with
    automatic AMI resolution
    ([guide](docs/guides/os-selection.md))
- **Multi-Architecture**: x86_64 and ARM64 with automatic architecture
    inference
- **Custom Templates**: Run user-provided scripts at any provisioning
    phase ([guide](docs/guides/custom-templates.md))
- **Multi-Node Clusters**: HA Kubernetes clusters with kubeadm
    ([guide](docs/guides/multinode-clusters.md))
- **Flexible Sources**: Install components from packages, git, runfiles,
    or latest branches
    ([guide](docs/guides/source-selection.md))
- **Automatic IP Detection**: No manual IP configuration needed for AWS
    ([guide](docs/guides/ip-detection.md))

---

## 🚀 Quick Start

See [docs/quick-start.md](docs/quick-start.md) for a full walkthrough.

### Install via Homebrew (macOS, Linux)

```bash
brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck
brew install nvidia/holodeck/holodeck
holodeck --help
```

Pre-built binaries for macOS (arm64, amd64) and Linux (arm64, amd64) are
downloaded from the [GitHub Releases page](https://github.com/NVIDIA/holodeck/releases/latest).
Run `brew upgrade nvidia/holodeck/holodeck` to update.

> **How the install works**
>
> Behind the scenes the tap ships two artifacts, both auto-bumped by
> GoReleaser on every release:
>
> | Platform | Mechanism | File |
> | --- | --- | --- |
> | macOS (arm64 / amd64) | Homebrew **Cask** | `Casks/holodeck.rb` |
> | Linux (arm64 / amd64) | Homebrew **Formula** | `Formula/holodeck.rb` |
>
> macOS uses a Cask because brew's Formula build-sandbox triggers a
> `PTY.open` failure on macOS Tahoe (26.x) + brew 5.1.x + portable-ruby
> 4.0.x. Casks skip that sandbox path, so `brew install` works cleanly.
> Until we wire up Apple Developer code signing + notarization, the Cask's
> `postflight` hook removes the `com.apple.quarantine` xattr so Gatekeeper
> doesn't block the unsigned binary.

### Install from source

```bash
make build
sudo mv ./bin/holodeck /usr/local/bin/holodeck
holodeck --help
```

---

## 🛠️ Prerequisites

- Go 1.20+
- (For AWS) Valid AWS credentials in your environment
- (For SSH) Reachable host and valid SSH key

See [docs/prerequisites.md](docs/prerequisites.md) for details.

---

## ⚠️ Important: Kernel Compatibility

When installing NVIDIA drivers, Holodeck requires kernel headers matching your running kernel
version. If exact headers are unavailable, Holodeck will attempt to find compatible ones,
though this may cause driver compilation issues.

For kernel compatibility details and troubleshooting, see
[Kernel Compatibility](docs/prerequisites.md#kernel-compatibility) in the prerequisites documentation.

---

## 📝 How to Contribute

See [docs/contributing/](docs/contributing/) for full details.

### Main Makefile Targets

- `make build` – Build the holodeck binary
- `make test` – Run all tests
- `make lint` – Run linters
- `make clean` – Remove build artifacts

---

## 🧑‍💻 Usage

See [docs/commands/](docs/commands/) for detailed command documentation and examples.

```bash
holodeck --help
```

### Example: Create an environment

```bash
holodeck create -f ./examples/v1alpha1_environment.yaml
```

### Example: List environments

```bash
holodeck list
```

### Example: Delete an environment

```bash
holodeck delete <instance-id>
```

### Example: Clean up AWS VPC resources

```bash
holodeck cleanup vpc-12345678
```

### Example: Check status

```bash
holodeck status <instance-id>
```

### Example: Dry Run

```bash
holodeck dryrun -f ./examples/v1alpha1_environment.yaml
```

### Remote-access kubeconfig (opt-in)

By default, the kubeconfig holodeck produces is configured for in-VM
use: file mode `0600` owned by the holodeck process user, and the
server URL points at the cluster's internal IP. To run `kubectl` from
outside the VPC (e.g., a GitHub Actions runner that provisioned the
cluster), set `kubernetes.remoteAccess: true`:

```yaml
spec:
  kubernetes:
    install: true
    installer: kubeadm
    remoteAccess: true
```

What changes when this is `true`:

- The kubeconfig server URL is rewritten to `https://<PublicDnsName>:6443`.
- The kubeconfig file is chowned to the bind-mounted workspace owner
  so the runner user (not just the action container's root) can read
  it. File mode stays `0600`.

What does **not** change:

- The security group still opens `6443` only to the auto-detected
  caller egress IP (`utils.GetIPAddress()`), not `0.0.0.0/0`.
- The embedded cluster admin cert is owner-only.

Platform: Linux/Darwin. On Windows, the chown step is a no-op.

**For downstream CI repos** (gpu-operator, k8s-device-plugin): set
`remoteAccess: true` in your `holodeck.yaml` and replace any
`rsync + ssh + remote-run` blocks in your workflow with a direct
`kubectl --kubeconfig=$GITHUB_WORKSPACE/kubeconfig …` step.

## Agentic skills

Holodeck ships an embedded catalog of agentic skills that teach an AI
coding agent how to drive the CLI correctly. List the catalog:

```bash
holodeck skill list
```

Install a skill into your AI agent's native format:

```bash
# Claude Code (project-local: ./.claude/skills/<name>/SKILL.md)
holodeck skill add --claude using-holodeck

# Multiple agents at once
holodeck skill add --claude --cursor --codex --gemini using-holodeck

# Or install everything for every agent, user-wide
holodeck skill add --all --all-agents --global
```

Supported agents: Claude Code, Cursor, Codex CLI, Gemini CLI. Skills
are short markdown guides authored against the actual CLI behavior;
they version with the code so updates land alongside the features
they describe.

## 📂 More

- [Examples](docs/examples/)
- [Guides](docs/guides/)

---

For more information, see the [documentation](docs/README.md) directory.
