# Contributing to Holodeck

Thank you for your interest in contributing to Holodeck! This guide will help
you get started.

## Development Setup

1. Fork the repository
1. Clone your fork:

   ```bash
   git clone https://github.com/your-username/holodeck.git
   cd holodeck
   ```

1. Add the upstream repository:

   ```bash
   git remote add upstream https://github.com/nvidia/holodeck.git
   ```

### Environment Requirements

- Linux or macOS (Windows is not supported)
- Go 1.20 or later
- Make
- Git

## Makefile Targets

The project uses a Makefile to manage common development tasks:

```bash
# Build the binary
make build

# Run tests
make test

# Run linters
make lint

# Clean build artifacts
make clean

# Run all checks (lint, test, build)
make check
```

## Running the CLI Locally

After building, you can run the CLI directly:

```bash
./bin/holodeck --help
```

Or install it system-wide:

```bash
sudo mv ./bin/holodeck /usr/local/bin/holodeck
```

## Development Workflow

1. Create a new branch for your feature/fix:

   ```bash
   git checkout -b feature/your-feature-name
   ```

1. Make your changes and commit them:

   ```bash
   git commit -s -m "feat: your feature description"
   ```

1. Push to your fork:

   ```bash
   git push origin feature/your-feature-name
   ```

1. Create a Pull Request against the main repository

## Commit Message Conventions

- Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
  - `feat: ...` for new features
  - `fix: ...` for bug fixes
  - `docs: ...` for documentation changes
  - `refactor: ...` for code refactoring
  - `test: ...` for adding or updating tests
  - `chore: ...` for maintenance
- Use the `-s` flag to sign off your commits

## Code Style

- Follow the Go code style guidelines
- Run `make lint` before submitting PRs
- Ensure all tests pass with `make test`

## Testing

- Write unit tests for new features
- Update existing tests when modifying features
- Run the full test suite with `make test`

## E2E Testing

Holodeck's end-to-end tests run on real AWS infrastructure. They are organized
into two tiers that control when tests execute in CI.

### E2E Test Structure

**Smoke tier (pre-merge)** — `.github/workflows/e2e-smoke.yaml`

Runs on every PR push. Covers two label filters:

- `default && !rpm` — standard single-node environment without RPM distros
- `cluster && minimal` — smallest valid multinode cluster

Each job takes roughly 20 minutes, giving fast feedback before merge.

**Full tier (post-merge)** — `.github/workflows/e2e.yaml`

Runs only when a commit lands on `main`
(`github.ref == 'refs/heads/main'`). Covers 13 label filters plus an
arm64 job and an integration-test job that exercises holodeck as a
GitHub Action.

| Label filter | What it covers |
|---|---|
| `legacy` | Kubernetes using a legacy version |
| `dra` | Dynamic Resource Allocation enabled |
| `kernel` | Kernel features / custom kernel |
| `ctk-git` | Container Toolkit installed from git source |
| `k8s-git` | Kubernetes built from git (kubeadm) |
| `k8s-kind-git` | Kubernetes built from git (KIND) |
| `k8s-latest` | Kubernetes tracking master branch |
| `cluster && gpu && !minimal && !ha && !dedicated` | Standard GPU cluster |
| `cluster && dedicated` | Cluster with dedicated CPU control-plane |
| `cluster && ha` | HA cluster (3 control-plane nodes) |
| `rpm-rocky` | Rocky Linux 9 — multiple container runtimes |
| `rpm-al2023` | Amazon Linux 2023 — multiple container runtimes |
| `rpm-fedora` | Fedora 42 — multiple container runtimes |
| `arm64` | ARM64 GPU instance (g5g) — run separately |

### Label Taxonomy

Tests are tagged with Ginkgo `Label()` annotations. Each test can carry
multiple labels; CI selects tests using boolean filter expressions.

**Single-node labels** (defined in `tests/aws_test.go`):

| Label | Description |
|---|---|
| `default` | Basic AWS environment, default configuration |
| `legacy` | Legacy Kubernetes version |
| `dra` | Dynamic Resource Allocation |
| `kernel` | Custom kernel features |
| `ctk-git` | CTK from git source |
| `k8s-git` | Kubernetes from git (kubeadm) |
| `k8s-kind-git` | Kubernetes from git (KIND) |
| `k8s-latest` | Kubernetes master branch |
| `rpm` | Any RPM-based distribution |
| `rpm-rocky` | Rocky Linux 9 |
| `rpm-al2023` | Amazon Linux 2023 |
| `rpm-fedora` | Fedora 42 |
| `post-merge` | Excluded from smoke tier; full tier only |

**Cluster labels** (defined in `tests/aws_cluster_test.go`):

| Label | Description |
|---|---|
| `cluster` | Multinode cluster test |
| `multinode` | Two or more nodes |
| `gpu` | GPU worker nodes |
| `minimal` | Smallest valid configuration (1 CP + 1 worker) |
| `dedicated` | Dedicated CPU control-plane node |
| `ha` | High-availability control plane (3 nodes) |
| `rpm` | RPM-based cluster OS |
| `rpm-rocky` | Rocky Linux 9 cluster |
| `rpm-al2023` | Amazon Linux 2023 cluster |
| `post-merge` | Excluded from smoke tier; full tier only |

The `post-merge` label is the mechanism that keeps a test out of the smoke
tier. The smoke workflow's label filter `"default && !rpm"` already excludes
RPM tests, but adding `post-merge` makes the intent explicit and ensures the
test is skipped by any future smoke filter that might otherwise match it.

### How to Add New Tests

1. **Single-node test** — add an `Entry(...)` to the `DescribeTable` in
   `tests/aws_test.go`.
1. **Cluster test** — add an `Entry(...)` to `tests/aws_cluster_test.go`.
1. Create the corresponding fixture file under `tests/data/`.
1. Assign Ginkgo labels with `Label("label1", "label2", ...)` as the last
   argument of the `Entry`.
1. If the test is an edge case, platform-specific variant, or is expensive
   (> 30 min), add `"post-merge"` to its label list so it runs only in the
   full tier.

Example:

```go
Entry("My New Feature Test", testConfig{
    name:        "my-feature-test",
    filePath:    filepath.Join(packagePath, "data", "test_aws_my_feature.yml"),
    description: "Tests my new feature end-to-end",
}, Label("default", "my-feature")),
```

### Which Tier to Target

| Use smoke tier (no `post-merge`) | Use full tier (`post-merge`) |
|---|---|
| Core functionality every PR should validate | Edge cases and less-common paths |
| Fast tests (< 25 min) | Slow tests (> 30 min) |
| Platform-agnostic defaults | Platform-specific variants (RPM distros, arm64) |
| Minimal cluster configurations | Full-scale, HA, or dedicated cluster topologies |

If in doubt, start with `post-merge` and promote the label out of the full
tier once the test has demonstrated stability.

### Running E2E Tests Locally

Use the Ginkgo label filter to select which tests to run:

```bash
# Run only the smoke-equivalent tests
make -f tests/Makefile test GINKGO_ARGS="--label-filter='default && !rpm'"

# Run a specific label
make -f tests/Makefile test GINKGO_ARGS="--label-filter='cluster && minimal'"

# Run all RPM tests for Rocky 9
make -f tests/Makefile test GINKGO_ARGS="--label-filter='rpm-rocky'"
```

Required environment variables:

```bash
export AWS_ACCESS_KEY_ID=<your-key-id>
export AWS_SECRET_ACCESS_KEY=<your-secret>
export E2E_SSH_KEY=<path-to-ssh-private-key>
```

See [Memory: no push without local E2E validation](../../.claude/memory/) for
the project policy on validating E2E tests before opening a PR.

## Documentation

- Update relevant documentation when adding features
- Follow the existing documentation style

## Pull Request Process

1. Ensure your PR description clearly describes the problem and solution
1. Include relevant issue numbers
1. Add tests for new functionality
1. Update documentation
1. Ensure CI passes

## Release Process

1. Version bump
1. Update changelog
1. Create release tag
1. Build and publish release artifacts

## Getting Help

- Open an issue for bugs or feature requests
- Join the community discussions
- Check existing documentation

## Code of Conduct

Please read and follow our [Code of Conduct](../CODE_OF_CONDUCT.md).
