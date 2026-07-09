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

Holodeck's end-to-end tests are Ginkgo specs tagged with `Label()`
annotations. Most drive `pkg/provider/aws` against a stateful in-memory AWS
fake (`internal/aws/awsfake`, credential-free); the rest drive it against
real AWS infrastructure. CI selects specs with boolean label-filter
expressions across three tiers — nothing runs in a scheduled or gated tier
unless its label appears in that tier's filter or matrix.

### E2E Test Structure

**PR tier** — `.github/workflows/e2e-smoke.yaml`, called from `ci.yaml` on
every push to a `pull-request/<N>`, `main`, or `release-*` branch:

- `e2e-mock` — the full mock suite (label `mock`), credential-free.
- `e2e-real-smoke` — one real-AWS job, `--label-filter='real-aws &&
  default && !rpm'`.

**Post-merge tier** — `.github/workflows/e2e.yaml`, called from `ci.yaml`
only when the push lands on `main` or a `release-*` branch:

- `e2e-mock` — the mock suite again (cheap; runs a second time on
  main/release pushes because both `e2e-smoke.yaml` and `e2e.yaml` fire on
  the same push).
- `e2e-real-full` — one real-AWS job, `--label-filter='real-aws &&
  cluster && minimal'`.
- `create-issue` — opens or updates a tracking issue (labels
  `e2e-failure`, `automated`) if either job fails.

**Weekly tier** — `.github/workflows/e2e-weekly.yaml`, on a Monday 06:00 UTC
cron or manual `workflow_dispatch`:

- `e2e-matrix` (schedule only) — the full real-AWS label matrix below.

| Matrix label | What it covers |
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
| `custom-template` | Custom instance template |
| `nvidia-driver` | NVIDIA driver installation path |

- `e2e-arm64` (schedule only) — `--label-filter='arm64'`. No spec
  currently carries `Label("arm64")`, so this job is a zero-spec no-op
  that exits green.
- `integration-test` (schedule only) — runs Holodeck itself as a GitHub
  Action (`NVIDIA/holodeck@main`) against `tests/data/test_aws.yml`.
- `e2e-dispatch` (manual only) — `workflow_dispatch` with a `label_filter`
  input; runs whatever Ginkgo label expression you provide.
- `create-issue` — opens or updates a tracking issue if `e2e-matrix`,
  `e2e-arm64`, or `integration-test` fails. Does not cover `e2e-dispatch`
  failures.

### The Mock Suite

`tests/e2e_mock_test.go` drives the real `pkg/provider/aws` `Create` /
`Delete` / `DryRun` code against `internal/aws/awsfake`, a stateful
in-memory fake EC2/SSM/ELBv2 implementation, instead of real AWS clients.
Every spec carries `Label("mock")` and needs no `AWS_*` credentials or
`E2E_SSH_KEY`. Coverage includes topology provisioning and teardown across
single-node and cluster configurations, rollback on injected API failures,
partial-cluster cleanup, and delete idempotency.

Run it locally:

```bash
env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u E2E_SSH_KEY \
  make -f tests/Makefile test GINKGO_ARGS="--label-filter='mock'"
```

### Label Taxonomy

Every real-AWS spec carries `Label("real-aws", ...)` plus one or more topic
labels. `requireRealAWSEnv()` (`tests/e2e_test.go`) gates those specs on
`E2E_SSH_KEY` being set; mock specs never call it.

**Single-node labels** (defined in `tests/aws_test.go`):

| Label | Description |
|---|---|
| `real-aws` | Provisions against real AWS; requires credentials |
| `default` | Basic AWS environment, default configuration |
| `legacy` | Legacy Kubernetes version |
| `dra` | Dynamic Resource Allocation |
| `kernel` | Custom kernel features |
| `nvidia-driver` | NVIDIA driver installation path |
| `ctk-git` | CTK from git source |
| `k8s-git` | Kubernetes from git (kubeadm) |
| `k8s-kind-git` | Kubernetes from git (KIND) |
| `k8s-latest` | Kubernetes master branch |
| `rpm` | Any RPM-based distribution |
| `rpm-rocky` | Rocky Linux 9 |
| `rpm-al2023` | Amazon Linux 2023 |
| `rpm-fedora` | Fedora 42 |
| `custom-template` | Custom instance template |
| `post-merge` | Present on several entries; currently informational only — no workflow filters on it |

**Cluster labels** (defined in `tests/aws_cluster_test.go`):

| Label | Description |
|---|---|
| `real-aws` | Provisions against real AWS; requires credentials |
| `cluster` | Multinode cluster test |
| `multinode` | Two or more nodes |
| `gpu` | GPU worker nodes |
| `minimal` | Smallest valid configuration (1 CP + 1 worker) |
| `dedicated` | Dedicated CPU control-plane node |
| `ha` | High-availability control plane (3 nodes) |
| `rpm` | RPM-based cluster OS |
| `rpm-rocky` | Rocky Linux 9 cluster |
| `rpm-al2023` | Amazon Linux 2023 cluster |
| `post-merge` | Present on several entries; currently informational only — no workflow filters on it |

**Mock label** (defined in `tests/e2e_mock_test.go`): `mock` —
credential-free, selected by the `e2e-mock` job in both `e2e-smoke.yaml`
and `e2e.yaml`.

### How to Add New Tests

1. **Single-node real-AWS test** — add an `Entry(...)` to the
   `DescribeTable` in `tests/aws_test.go`.
1. **Cluster real-AWS test** — add an `Entry(...)` to
   `tests/aws_cluster_test.go`.
1. **Mock test** — add a spec to `tests/e2e_mock_test.go`, driving
   `pkg/provider/aws` against `internal/aws/awsfake` (see the
   `newMockProvider` helper).
1. Create the corresponding fixture file under `tests/data/`.
1. Every real-AWS `Entry` needs `Label("real-aws", <topic labels...>)` as
   its last argument. The topic labels determine which CI tiers pick it
   up — a label is only run in a scheduled or gated tier if that tier's
   workflow selects it:
  - Listed in `e2e-weekly.yaml`'s `e2e-matrix` label list → runs weekly.
  - Matches `default && !rpm` → also runs in the PR smoke job.
  - Matches `cluster && minimal` → also runs in the post-merge job.
  - Otherwise it is reachable only via manual `workflow_dispatch`
    (`e2e-dispatch`) with a matching `label_filter`.

   Nothing is selected implicitly: a new topic label needs an explicit
   entry in a workflow filter or matrix to run anywhere but manual dispatch.
1. `post-merge` is currently informational only — no workflow filters on
   it. Do not add it expecting it to keep a test out of any tier; control
   routing with the topic labels above instead.

Example:

```go
Entry("My New Feature Test", testConfig{
    name:        "my-feature-test",
    filePath:    filepath.Join(packagePath, "data", "test_aws_my_feature.yml"),
    description: "Tests my new feature end-to-end",
}, Label("real-aws", "default", "my-feature")),
```

### Running E2E Tests Locally

Mock suite (no credentials needed):

```bash
env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u E2E_SSH_KEY \
  make -f tests/Makefile test GINKGO_ARGS="--label-filter='mock'"
```

Real-AWS tests need credentials and provision real GPU instances:

```bash
export AWS_ACCESS_KEY_ID=<your-key-id>
export AWS_SECRET_ACCESS_KEY=<your-secret>
export E2E_SSH_KEY=<path-to-ssh-private-key>

# Run the PR-smoke-equivalent filter
make -f tests/Makefile test GINKGO_ARGS="--label-filter='real-aws && default && !rpm'"

# Run a specific weekly-matrix label
make -f tests/Makefile test GINKGO_ARGS="--label-filter='rpm-rocky'"
```

`tests/run_cluster_e2e.sh` still works for cluster labels (`minimal`,
`ha`, `dedicated`, `gpu`, `multinode`, `cluster`); it exports
`E2E_SSH_KEY` itself, so `requireRealAWSEnv()` passes.

> **Important:** Always validate E2E tests locally before pushing. CI E2E
> runs provision real GPU instances on AWS, and unnecessary runs are expensive.

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
