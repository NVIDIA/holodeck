# NVIDIA Driver Installation Sources

Holodeck supports installing the NVIDIA GPU driver from multiple sources to
accommodate different use cases - from production deployments to kernel module
development.

## Available Sources

### Package (Default)

Installs the driver from official CUDA repository packages. This is the
recommended source for production environments.

```yaml
spec:
  nvidiaDriver:
    install: true
    source: package
    package:
      branch: "560"         # driver branch (e.g., 560, 550, 545)
      version: "560.35.03"  # optional exact version pin
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `branch` | Driver branch to install (e.g., `560`, `550`) | `575` |
| `version` | Exact package version to install | Latest in branch |

When `version` is specified, it takes precedence over `branch` for package
selection.

### Runfile

Installs the driver from an NVIDIA `.run` installer file. Useful for testing
pre-release drivers or specific builds not available in package repositories.

```yaml
spec:
  nvidiaDriver:
    install: true
    source: runfile
    runfile:
      url: https://download.nvidia.com/XFree86/Linux-x86_64/560.35.03/NVIDIA-Linux-x86_64-560.35.03.run
      checksum: "sha256:abc123..."  # optional SHA256 checksum
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `url` | Download URL for the `.run` file (required) | - |
| `checksum` | SHA256 checksum for verification (e.g., `sha256:abc...`) | Not verified |

The runfile is installed with `--silent --dkms` flags for unattended operation
with DKMS kernel module management.

### Git

Builds and installs the driver from the
[open-gpu-kernel-modules](https://github.com/NVIDIA/open-gpu-kernel-modules)
repository. Use this when you need to test specific commits, branches, or
patches to the open kernel modules.

```yaml
spec:
  nvidiaDriver:
    install: true
    source: git
    git:
      ref: "560.35.03"  # tag, commit SHA, or branch
      repo: https://github.com/NVIDIA/open-gpu-kernel-modules.git  # optional
```

**Configuration options:**

| Field | Description | Default |
|-------|-------------|---------|
| `ref` | Git reference (required) - tag, commit SHA, or branch | - |
| `repo` | Git repository URL | Official NVIDIA repo |

**Supported ref formats:**

- Tags: `560.35.03`, `refs/tags/560.35.03`
- Branches: `main`, `refs/heads/main`
- Commits: `abc123def456`

**Build process:**

1. Clones the repository at the specified ref
2. Builds kernel modules with `make modules -j$(nproc)`
3. Installs with `make modules_install`
4. Runs `depmod` and loads the `nvidia` module

> **Note:** Git source requires kernel headers and a C compiler on the target
> host. These are installed automatically. Building kernel modules can take
> several minutes depending on the instance type.

## Provenance Tracking

All installation sources write provenance information to
`/etc/nvidia-driver/PROVENANCE.json`. This file contains:

- Source type (`package`, `runfile`, or `git`)
- Version or commit information
- Installation timestamp
- For runfile sources: download URL and checksum
- For git sources: repository URL and ref

## Examples

### Production: Pinned Package Version

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  nvidiaDriver:
    install: true
    source: package
    package:
      version: "560.35.03"
```

### Testing: Specific Driver Branch

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: branch-test
spec:
  nvidiaDriver:
    install: true
    source: package
    package:
      branch: "550"
```

### Pre-release: Runfile Installer

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: prerelease-test
spec:
  nvidiaDriver:
    install: true
    source: runfile
    runfile:
      url: https://download.nvidia.com/XFree86/Linux-x86_64/560.35.03/NVIDIA-Linux-x86_64-560.35.03.run
      checksum: "sha256:abc123..."
```

### Development: Build from Source

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: driver-dev
spec:
  nvidiaDriver:
    install: true
    source: git
    git:
      ref: refs/heads/main
```

## Backward Compatibility

The legacy `branch` and `version` fields at the top level are still supported:

```yaml
spec:
  nvidiaDriver:
    install: true
    branch: "560"         # Legacy, equivalent to package.branch
    version: "560.35.03"  # Legacy, equivalent to package.version
```

When `source` is not specified, it defaults to `package`.
