# NVIDIA Container Toolkit Installation Sources

Holodeck supports installing the NVIDIA Container Toolkit (CTK) from multiple
sources to accommodate different use cases - from production deployments to
development testing.

## Available Sources

### Package (Default)

Installs CTK from official NVIDIA distribution packages. This is the
recommended source for production environments.

```yaml
spec:
  nvidiaContainerToolkit:
    install: true
    source: package
    package:
      channel: stable      # or "experimental"
      version: "1.17.3-1"  # optional, omit for latest
    enableCDI: true
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `channel` | Package channel (`stable` or `experimental`) | `stable` |
| `version` | Specific package version to install | Latest available |

### Git

Installs CTK from a specific git reference (tag, commit, branch). Holodeck
first attempts to pull pre-built packages from GHCR, falling back to building
from source if unavailable.

```yaml
spec:
  nvidiaContainerToolkit:
    install: true
    source: git
    git:
      ref: v1.17.3  # tag, commit SHA, or branch
      repo: https://github.com/NVIDIA/nvidia-container-toolkit.git  # optional
    enableCDI: true
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `ref` | Git reference (required) - tag, commit SHA, or branch | - |
| `repo` | Git repository URL | Official NVIDIA repo |

**Supported ref formats:**

- Tags: `v1.17.3`, `refs/tags/v1.17.3`
- Branches: `main`, `refs/heads/release-1.17`
- Commits: `abc123def456`
- Pull requests: `refs/pull/123/head`

### Latest

Tracks a moving branch at provision time. Each provisioning will fetch the
latest commit from the specified branch.

```yaml
spec:
  nvidiaContainerToolkit:
    install: true
    source: latest
    latest:
      track: main          # branch to track
      repo: https://github.com/NVIDIA/nvidia-container-toolkit.git  # optional
    enableCDI: true
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `track` | Branch name to track | `main` |
| `repo` | Git repository URL | Official NVIDIA repo |

## Provenance Tracking

All installation sources write provenance information to
`/etc/nvidia-container-toolkit/PROVENANCE.json`. This file contains:

- Source type (`package`, `git`, or `latest`)
- Version or commit information
- Installation timestamp
- For git sources: repository URL and GHCR digest (if applicable)

Example provenance file:

```json
{
  "source": "git",
  "repo": "https://github.com/NVIDIA/nvidia-container-toolkit.git",
  "ref": "v1.17.3",
  "commit": "abc12345",
  "ghcr_digest": "sha256:...",
  "installed_at": "2026-01-09T10:30:00-05:00"
}
```

## Examples

### Production: Pinned Package Version

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  nvidiaContainerToolkit:
    install: true
    source: package
    package:
      channel: stable
      version: "1.17.3-1"
    enableCDI: true
```

### Testing: Specific Release Tag

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: release-test
spec:
  nvidiaContainerToolkit:
    install: true
    source: git
    git:
      ref: refs/tags/v1.17.3
    enableCDI: true
```

### Development: Track Main Branch

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: dev-latest
spec:
  nvidiaContainerToolkit:
    install: true
    source: latest
    latest:
      track: main
    enableCDI: true
```

### Debugging: Specific Commit

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: debug-issue
spec:
  nvidiaContainerToolkit:
    install: true
    source: git
    git:
      ref: abc123def456
    enableCDI: true
```

## Backward Compatibility

The legacy `version` field is still supported for package installations:

```yaml
spec:
  nvidiaContainerToolkit:
    install: true
    version: "1.17.3-1"  # Legacy field, equivalent to package.version
```

When `source` is not specified, it defaults to `package`.
