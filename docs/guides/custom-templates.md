# Custom Templates

Custom templates let you run user-provided scripts at specific points
during Holodeck provisioning. This enables customizations such as
installing additional packages, configuring monitoring, or applying
security policies without modifying Holodeck itself.

## Source Types

Each custom template must specify **exactly one** source:

| Source   | Description                                    | Field    |
|----------|------------------------------------------------|----------|
| Inline   | Script content embedded directly in YAML       | `inline` |
| File     | Path to a local script file                    | `file`   |
| URL      | HTTPS URL to fetch the script from             | `url`    |

Specifying zero sources or more than one source is a validation error
that is caught before provisioning starts.

## Execution Phases

Custom templates execute at one of five phases during provisioning:

| Phase             | When it runs                                  | Typical use case                         |
|-------------------|-----------------------------------------------|------------------------------------------|
| `pre-install`     | Before any Holodeck components (kernel, etc.) | System prerequisites, package repos      |
| `post-runtime`    | After container runtime installation          | Runtime tuning, registry mirrors         |
| `post-toolkit`    | After NVIDIA Container Toolkit installation   | CDI configuration, GPU validation        |
| `post-kubernetes` | After Kubernetes is ready                     | Workload deployment, cluster config      |
| `post-install`    | After all components (default)                | End-to-end validation, smoke tests       |

If no phase is specified, the template defaults to `post-install`.

Multiple templates can share the same phase and execute in the
order they appear in the YAML.

## Field Reference

| Field             | Type              | Required | Default        | Description                                      |
|-------------------|-------------------|----------|----------------|--------------------------------------------------|
| `name`            | string            | yes      | -              | Human-readable identifier; must be unique        |
| `phase`           | string            | no       | `post-install` | Execution phase (see table above)                |
| `inline`          | string            | one of   | -              | Script content embedded in YAML                  |
| `file`            | string            | one of   | -              | Path to a local script file                      |
| `url`             | string            | one of   | -              | HTTPS URL to fetch the script from               |
| `checksum`        | string            | no       | -              | SHA256 checksum for `file` or `url` sources      |
| `timeout`         | int               | no       | `600`          | Maximum execution time in seconds                |
| `continueOnError` | bool              | no       | `false`        | Continue provisioning if the script fails        |
| `env`             | map[string]string | no       | -              | Environment variables exported before the script |

## Configuration Options

```yaml
customTemplates:
  - name: my-script          # Required: human-readable identifier
    phase: post-install       # Optional: execution phase (default: post-install)
    inline: |                 # Source: inline script content
      #!/bin/bash
      echo "Hello from custom template"
    env:                      # Optional: environment variables
      MY_VAR: value
    timeout: 600              # Optional: timeout in seconds (default: 600)
    continueOnError: false    # Optional: continue provisioning on failure (default: false)
    checksum: "sha256:..."    # Optional: SHA256 checksum for verification
```

### File Source

```yaml
customTemplates:
  - name: setup-monitoring
    phase: post-kubernetes
    file: ./scripts/monitoring.sh
```

Relative paths are resolved from the directory containing the
Holodeck configuration file. Absolute paths are used as-is. Path
traversal (`..`) components are rejected during validation.

### URL Source

```yaml
customTemplates:
  - name: install-tools
    phase: post-install
    url: https://example.com/scripts/install-tools.sh
    checksum: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
```

URLs **must** use the `https://` scheme. HTTP and other schemes are
rejected by the validator. Always provide a `checksum` for URL sources
to verify script integrity before execution. Scripts larger than 10 MB
are rejected.

## Examples

### Pre-install Repository Configuration

```yaml
customTemplates:
  - name: configure-repos
    phase: pre-install
    inline: |
      #!/bin/bash
      apt-get update
      apt-get install -y ca-certificates curl gnupg
```

### Post-Kubernetes Monitoring Stack

```yaml
customTemplates:
  - name: install-prometheus
    phase: post-kubernetes
    inline: |
      #!/bin/bash
      kubectl create namespace monitoring || true
      helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
      helm install prometheus prometheus-community/prometheus -n monitoring
    env:
      KUBECONFIG: /etc/kubernetes/admin.conf
    continueOnError: true
    timeout: 300
```

### Multiple Templates Across Phases

```yaml
customTemplates:
  - name: system-prep
    phase: pre-install
    inline: |
      #!/bin/bash
      sysctl -w vm.max_map_count=262144

  - name: runtime-tuning
    phase: post-runtime
    file: ./scripts/tune-containerd.sh

  - name: deploy-app
    phase: post-kubernetes
    url: https://internal.example.com/deploy.sh
    checksum: "sha256:abc123..."

  - name: smoke-test
    phase: post-install
    inline: |
      #!/bin/bash
      kubectl get nodes
      kubectl get pods -A
    continueOnError: true
```

### Complete Environment with Custom Templates

The following is a full environment spec that installs the NVIDIA driver,
containerd, the Container Toolkit, and Kubernetes — then runs custom
scripts at three phases:

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: gpu-cluster-with-customizations
spec:
  provider: aws
  auth:
    keyName: my-key-pair
    publicKey: ~/.ssh/id_rsa.pub
    privateKey: ~/.ssh/id_rsa
  instance:
    type: g4dn.xlarge
    region: us-east-1
    os: ubuntu-22.04
  nvidiaDriver:
    install: true
  containerRuntime:
    install: true
    name: containerd
  nvidiaContainerToolkit:
    install: true
  kubernetes:
    install: true
  customTemplates:
    # Phase 1: pre-install — run before any component is installed
    - name: configure-apt-proxy
      phase: pre-install
      inline: |
        #!/bin/bash
        echo 'Acquire::http::Proxy "http://proxy.example.com:3128";' \
          > /etc/apt/apt.conf.d/99proxy

    # Phase 2: post-toolkit — run after CTK, before Kubernetes
    - name: configure-cdi
      phase: post-toolkit
      inline: |
        #!/bin/bash
        nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml
        nvidia-ctk runtime configure --runtime=containerd --cdi.enabled

    # Phase 3: post-kubernetes — deploy workloads once cluster is ready
    - name: deploy-gpu-operator
      phase: post-kubernetes
      url: https://scripts.example.com/deploy-gpu-operator.sh
      checksum: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      env:
        OPERATOR_VERSION: v24.9.0
        NAMESPACE: gpu-operator
      timeout: 600
      continueOnError: false
```

## Security Considerations

- **Checksum verification**: Use `checksum` with URL sources to
  verify script integrity.
- **HTTPS required**: URL sources must use HTTPS. Non-HTTPS URLs
  are rejected during validation.
- **`continueOnError`**: Use carefully. When enabled, a failing custom
  template will not stop provisioning, which may leave the system in a
  partially configured state.
- **File paths**: Relative file paths are resolved from the config
  directory. Avoid path traversal patterns.

## Troubleshooting

### Template fails with "no such file or directory"

- **Cause:** File path is relative but `baseDir` was not set, or the
    file doesn't exist at the resolved path.
- **Fix:** Use absolute paths or ensure the file exists relative to
    the Holodeck config file directory.

### Template fails with "checksum mismatch"

- **Cause:** The downloaded or read content doesn't match the
    expected SHA256 hash.
- **Fix:** Regenerate the checksum:
    `sha256sum script.sh | awk '{print "sha256:" $1}'`

### Template fails with "invalid environment variable name"

- **Cause:** Env var keys must match POSIX shell rules:
    start with letter or underscore, contain only letters, digits,
    and underscores.
- **Fix:** Rename the variable (e.g., `my-var` -> `MY_VAR`).

### Validation error "duplicate name"

- **Cause:** Two templates in `customTemplates` share the same `name` value.
    Names must be unique across all templates, regardless of phase.
- **Fix:** Give each template a distinct `name`.

### Validation error "exactly one source must be specified"

- **Cause:** A template has either no source fields (`inline`, `file`, `url`)
    or more than one set simultaneously.
- **Fix:** Set exactly one of `inline`, `file`, or `url`.

### Dryrun passes but template fails at runtime

- **Cause:** Dryrun validates configuration but does not execute
    scripts. Runtime failures (missing commands, network issues) are
    only caught during provisioning.
- **Fix:** Test scripts locally before adding to Holodeck config.
    Use `continueOnError: true` for non-critical scripts.

## Best Practices

1. **Use `pre-install` for system prerequisites** like package
   repos, kernel parameters, or certificates.
1. **Use `post-kubernetes` for workload deployment** since the
   cluster is ready at that point.
1. **Use `post-install` for validation scripts** that verify
   the full stack.
1. **Set `continueOnError: true` for non-critical scripts**
   like monitoring or logging.
1. **Add checksums for URL sources** to ensure script integrity.
1. **Keep scripts idempotent** so re-runs produce the same result.
1. **Test with `holodeck dryrun`** to validate configuration
   before provisioning.

## Related

- [Examples: Custom Templates](../../examples/custom_templates.yaml)
- [Dryrun Command](../commands/dryrun.md) — validates custom template
    configuration
- [Source Selection Guide](source-selection.md) — choosing installation
    sources for other components
