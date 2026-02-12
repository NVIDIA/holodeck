# Source Selection Guide

This guide helps you choose the right installation source for each component
in your Holodeck environment.

## Quick Reference

| Use Case | Driver | Runtime | Toolkit | Kubernetes |
|----------|--------|---------|---------|------------|
| Production | `package` | `package` | `package` | `release` |
| Version pinning | `package` (version) | `package` (version) | `package` (version) | `release` (version) |
| Pre-release testing | `runfile` | `git` (tag) | `git` (tag) | `git` (tag) |
| Regression bisection | `git` (commit) | `git` (commit) | `git` (commit) | `git` (commit) |
| CI/CD latest | N/A | `latest` | `latest` | `latest` |
| Custom kernel modules | `git` | N/A | N/A | N/A |

## When to Use Each Source

### Package (Recommended Default)

**Use when:**

- Deploying production or stable test environments
- You need well-tested, signed packages
- Reproducibility via version pinning is sufficient
- You want the fastest provisioning time

**Trade-offs:**

- (+) Fastest installation
- (+) Pre-built, tested packages
- (+) Automatic dependency resolution
- (-) Limited to released versions
- (-) No access to unreleased fixes

### Git

**Use when:**

- Testing a specific commit or unreleased fix
- Bisecting a regression across commits
- Validating a PR before it merges
- Testing a fork with custom modifications

**Trade-offs:**

- (+) Pin to any commit, tag, or branch
- (+) Test unreleased code
- (+) Use custom forks
- (-) Slower (requires build from source)
- (-) Build may fail on incompatible hosts
- (-) Requires build tools on target host

### Latest

**Use when:**

- CI/CD pipelines testing against upstream HEAD
- Continuous compatibility testing
- Catching regressions early in upstream development

**Trade-offs:**

- (+) Always tests newest code
- (+) Catches breaking changes early
- (-) Non-reproducible (branch moves)
- (-) May encounter broken builds
- (-) Slower (build from source)

### Runfile (Driver Only)

**Use when:**

- Testing pre-release driver builds from NVIDIA
- Using driver versions not in any package repository
- Validating specific installer artifacts

**Trade-offs:**

- (+) Access to any published `.run` file
- (+) Checksum verification for integrity
- (-) Must manage URLs manually
- (-) No automatic dependency resolution

## Example Configurations

### All Pinned (Reproducible)

Best for: CI/CD baselines, regression testing.

```yaml
spec:
  nvidiaDriver:
    install: true
    source: package
    package:
      version: "560.35.03"
  containerRuntime:
    install: true
    name: containerd
    source: package
    package:
      version: "1.7.23"
  nvidiaContainerToolkit:
    install: true
    source: package
    package:
      version: "1.17.3-1"
  kubernetes:
    install: true
    source: release
    release:
      version: v1.31.1
```

### Mixed: Stable Stack + Dev Component

Best for: Testing one component against a known-good stack.

```yaml
spec:
  nvidiaDriver:
    install: true
    # Default package source, latest stable
  containerRuntime:
    install: true
    name: containerd
    source: git
    git:
      ref: v1.7.23  # Test specific containerd version
  nvidiaContainerToolkit:
    install: true
    source: latest
    latest:
      track: main  # Test CTK against latest
  kubernetes:
    install: true
    source: release
    release:
      version: v1.31.1
```

### All Latest (Bleeding Edge)

Best for: Compatibility testing, catching regressions early.

```yaml
spec:
  nvidiaDriver:
    install: true
    source: git
    git:
      ref: refs/heads/main
  containerRuntime:
    install: true
    name: containerd
    source: latest
    latest:
      track: main
  nvidiaContainerToolkit:
    install: true
    source: latest
    latest:
      track: main
  kubernetes:
    install: true
    source: latest
    latest:
      track: master
```

## Component-Specific Documentation

- [NVIDIA Driver Sources](driver-sources.md)
- [Container Runtime Sources](runtime-sources.md)
- [NVIDIA Container Toolkit Sources](ctk-sources.md)
