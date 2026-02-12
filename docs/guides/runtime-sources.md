# Container Runtime Installation Sources

Holodeck supports installing container runtimes (containerd, Docker, CRI-O)
from multiple sources to enable testing different versions and development
builds.

## Available Sources

### Package (Default)

Installs the runtime from official distribution packages. This is the
recommended source for production environments.

```yaml
spec:
  containerRuntime:
    install: true
    name: containerd
    source: package
    package:
      version: "1.7.23"  # optional, omit for latest
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `version` | Specific package version to install | Latest stable |

**Default versions by runtime:**

| Runtime | Default Version | Package Source |
|---------|----------------|----------------|
| containerd (v1.x) | 1.7.27 | Docker repository (`containerd.io`) |
| containerd (v2.x) | Binary from GitHub releases | Official releases |
| Docker | latest | Docker repository (`docker-ce`) |
| CRI-O | latest | pkgs.k8s.io repository |

### Git

Installs the runtime from a specific git reference by building from source.
Use this when you need to test specific commits, branches, or unreleased
features.

```yaml
spec:
  containerRuntime:
    install: true
    name: containerd
    source: git
    git:
      ref: v1.7.23  # tag, commit SHA, or branch
      repo: https://github.com/containerd/containerd.git  # optional
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `ref` | Git reference (required) - tag, commit SHA, or branch | - |
| `repo` | Git repository URL | Upstream repo for selected runtime |

**Default repositories by runtime:**

| Runtime | Default Repository |
|---------|-------------------|
| containerd | `https://github.com/containerd/containerd.git` |
| Docker | `https://github.com/moby/moby.git` |
| CRI-O | `https://github.com/cri-o/cri-o.git` |

**Build process:**

All runtimes are built from source using Go. Holodeck automatically installs
the Go toolchain on the target host if not present.

- **containerd:** `make && make install`, plus runc and CNI plugins
- **Docker/moby:** `hack/make.sh binary`, plus cri-dockerd for K8s compat
- **CRI-O:** `make && make install`, plus conmon and CNI plugins

### Latest

Tracks a moving branch at provision time. Each provisioning resolves the
branch to its current HEAD commit and builds from source.

```yaml
spec:
  containerRuntime:
    install: true
    name: containerd
    source: latest
    latest:
      track: main  # branch to track
      repo: https://github.com/containerd/containerd.git  # optional
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `track` | Branch name to track | `main` |
| `repo` | Git repository URL | Upstream repo for selected runtime |

> **Note:** The `latest` source is available for containerd only. Docker and
> CRI-O support `package` and `git` sources.

## Provenance Tracking

Git and latest sources write provenance information to the runtime's config
directory:

- containerd: `/etc/containerd/PROVENANCE.json`
- Docker: `/etc/docker/PROVENANCE.json`
- CRI-O: `/etc/crio/PROVENANCE.json`

## Examples

### Production: Pinned containerd Version

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  containerRuntime:
    install: true
    name: containerd
    source: package
    package:
      version: "1.7.23"
```

### Testing: containerd v2.x

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: containerd-v2
spec:
  containerRuntime:
    install: true
    name: containerd
    source: package
    package:
      version: "2.0.0"
```

### Development: containerd from Source

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: containerd-dev
spec:
  containerRuntime:
    install: true
    name: containerd
    source: git
    git:
      ref: v1.7.23
```

### CI: Track containerd Main Branch

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: containerd-latest
spec:
  containerRuntime:
    install: true
    name: containerd
    source: latest
    latest:
      track: main
```

### Testing: CRI-O from Git Tag

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: crio-test
spec:
  containerRuntime:
    install: true
    name: crio
    source: git
    git:
      ref: v1.30.0
```

### Testing: Moby/Docker from Source

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: docker-dev
spec:
  containerRuntime:
    install: true
    name: docker
    source: git
    git:
      ref: v24.0.0
```

## Backward Compatibility

The legacy `version` field at the top level is still supported:

```yaml
spec:
  containerRuntime:
    install: true
    name: containerd
    version: "1.7.23"  # Legacy, equivalent to package.version
```

When `source` is not specified, it defaults to `package`.
