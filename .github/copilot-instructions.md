# Copilot Instructions for Holodeck

This document provides context and guidelines for AI assistants (GitHub Copilot,
Cursor, Claude, etc.) working on the Holodeck codebase.

---

## Project Overview

**Holodeck** is a CLI tool for creating and managing GPU-ready cloud test
environments. It automates the provisioning of:

- Cloud infrastructure (AWS EC2 instances)
- NVIDIA GPU drivers
- Container runtimes (Docker, containerd, CRI-O)
- NVIDIA Container Toolkit
- Kubernetes clusters (kubeadm, Kind)

### Primary Use Cases

1. **CI/CD Testing**: Spin up GPU environments for automated testing
2. **Development**: Create ephemeral GPU development environments
3. **Validation**: Test NVIDIA software stack configurations

---

## Repository Structure

```
holodeck/
├── api/holodeck/v1alpha1/    # Kubernetes-style API types (Environment CRD)
├── cmd/
│   ├── cli/                  # Main CLI application
│   │   ├── create/           # holodeck create command
│   │   ├── delete/           # holodeck delete command
│   │   ├── dryrun/           # holodeck dryrun command
│   │   ├── list/             # holodeck list command
│   │   └── status/           # holodeck status command
│   └── action/               # GitHub Action entrypoint
├── pkg/
│   ├── provider/             # Cloud provider interface and implementations
│   │   └── aws/              # AWS EC2 provider
│   ├── provisioner/          # Software provisioning logic
│   │   └── templates/        # Provisioning templates for each component
│   ├── jyaml/                # YAML utilities with JSON schema support
│   └── utils/                # Shared utilities
├── examples/                 # Example environment YAML files
├── docs/                     # Documentation
└── tests/                    # E2E tests
```

---

## Key Concepts

### Environment API

The `Environment` CRD (Custom Resource Definition) defines the desired state:

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-gpu-env
spec:
  provider: aws  # or "ssh" for existing instances
  auth:
    keyName: my-key
    username: ubuntu
    publicKey: ~/.ssh/id_rsa.pub
    privateKey: ~/.ssh/id_rsa
  instance:
    type: g4dn.xlarge
    region: us-west-2
    image:
      architecture: x86_64
  nvidiaDriver:
    install: true
    branch: "535"
  containerRuntime:
    install: true
    name: containerd
  nvidiaContainerToolkit:
    install: true
  kubernetes:
    install: true
    Installer: kubeadm
```

### Providers

- **AWS Provider** (`pkg/provider/aws/`): Creates EC2 instances with proper
  security groups, key pairs, and GPU support.
- **SSH Provider**: Uses existing instances accessible via SSH.

### Provisioner

The provisioner (`pkg/provisioner/`) executes shell commands over SSH to install
and configure software. Each component has a template in
`pkg/provisioner/templates/`:

- `nv-driver.go` - NVIDIA driver installation
- `containerd.go`, `docker.go`, `crio.go` - Container runtimes
- `container-toolkit.go` - NVIDIA Container Toolkit
- `kubernetes.go` - Kubernetes cluster setup
- `kernel.go` - Kernel version management

---

## Coding Conventions

### Go Style

- Follow standard Go conventions and `gofmt`
- Use `golangci-lint` for linting (configured in `.golangci.yml` if present)
- Documentation comments should wrap at 80 characters
- Error messages should be lowercase, no trailing punctuation
- Use `context.Context` for cancellation and timeouts

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create instance: %w", err)
}

// Good: Use structured errors where appropriate
var ErrInstanceNotFound = errors.New("instance not found")
```

### Logging

Use structured logging with the project's logger (`internal/logger/`):

```go
logger.Info("Creating instance", "type", instanceType, "region", region)
logger.Error("Failed to provision", "error", err)
```

### Testing

- Unit tests alongside source files: `foo_test.go`
- E2E tests in `tests/` directory
- Use testify for assertions when appropriate
- Table-driven tests preferred for multiple cases

```go
func TestCreateInstance(t *testing.T) {
    tests := []struct {
        name    string
        spec    InstanceSpec
        want    *Instance
        wantErr bool
    }{
        // test cases...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

---

## Common Patterns

### Adding a New Provisioner Template

1. Create `pkg/provisioner/templates/mycomponent.go`
2. Implement the `Template` interface
3. Register in the dependency graph (`pkg/provisioner/dependency.go`)
4. Add tests in `pkg/provisioner/templates/mycomponent_test.go`

### Adding a New CLI Command

1. Create `cmd/cli/mycommand/mycommand.go`
2. Define the command using `urfave/cli`
3. Register in `cmd/cli/main.go`
4. Add documentation in `docs/commands/mycommand.md`

### Adding a New Provider

1. Implement `pkg/provider.Provider` interface
2. Create package in `pkg/provider/myprovider/`
3. Add provider constant in `api/holodeck/v1alpha1/types.go`
4. Update provider factory in `pkg/provider/provider.go`

---

## Dependencies

Key dependencies to be aware of:

- `github.com/urfave/cli/v2` - CLI framework
- `github.com/aws/aws-sdk-go-v2` - AWS SDK
- `k8s.io/apimachinery` - Kubernetes API machinery (for types)
- `github.com/pkg/sftp` - SFTP for file transfers
- `golang.org/x/crypto/ssh` - SSH client

---

## Build and Test Commands

```bash
# Build the binary
make build

# Run all tests
make test

# Run linter
make lint

# Run E2E tests (requires AWS credentials)
make e2e

# Clean build artifacts
make clean
```

---

## Important Files

| File | Purpose |
|------|---------|
| `api/holodeck/v1alpha1/types.go` | Environment CRD definition |
| `pkg/provider/provider.go` | Provider interface |
| `pkg/provisioner/provisioner.go` | Main provisioning orchestrator |
| `pkg/provisioner/dependency.go` | Component dependency ordering |
| `cmd/cli/main.go` | CLI entrypoint and command registration |
| `action.yml` | GitHub Action definition |

---

## Security Considerations

- Never log sensitive data (SSH keys, AWS credentials)
- Use `os.FileMode(0600)` for private key files
- Validate all user inputs from YAML configurations
- Use secure defaults for security groups (minimal ingress)
- SSH connections should use known host verification in production

---

## Common Pitfalls

1. **SSH Connection Issues**: Ensure security groups allow SSH (port 22) from
   the machine running Holodeck.

2. **GPU Driver Compatibility**: The NVIDIA driver version must be compatible
   with the kernel version. Check the compatibility matrix.

3. **AWS Quotas**: GPU instances have quota limits. Check Service Quotas in
   AWS console if instance creation fails.

4. **Kubernetes Version Skew**: Ensure kubelet, kubeadm, and kubectl versions
   match within the allowed skew policy.

---

## When Making Changes

1. **Check dependencies**: Use `pkg/provisioner/dependency.go` to understand
   component ordering.

2. **Test locally first**: Use `holodeck dryrun` to validate YAML without
   creating resources.

3. **Update documentation**: If adding features, update relevant docs in
   `docs/` directory.

4. **Sign commits**: All commits must be signed off (DCO):
   ```bash
   git commit -s -m "feat: add new feature"
   ```

5. **Follow conventional commits**:
   - `feat:` - New features
   - `fix:` - Bug fixes
   - `docs:` - Documentation
   - `refactor:` - Code refactoring
   - `test:` - Test additions/changes
   - `chore:` - Maintenance tasks

---

## Getting Help

- Check existing issues and PRs for context
- Review `docs/` directory for detailed documentation
- Look at `examples/` for usage patterns
- Check test files for expected behavior
