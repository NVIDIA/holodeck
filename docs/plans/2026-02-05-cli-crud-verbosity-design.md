# CLI CRUD Operations & Verbosity Design

**Issue:** #563 - Complete CLI with Full CRUD Operations
**Date:** 2026-02-05
**Status:** Draft

## Summary

This design covers two areas:
1. New CLI commands for CRUD operations (already implemented, needs tests)
2. Global verbosity levels for consistent output control

## Part 1: CLI CRUD Commands (Implemented)

### New Commands

| Command | Description | Status |
|---------|-------------|--------|
| `describe` | Detailed instance introspection | Implemented |
| `get kubeconfig` | Download kubeconfig from instance | Implemented |
| `get ssh-config` | Generate SSH config entry | Implemented |
| `ssh` | SSH into instance or run commands | Implemented |
| `scp` | Copy files to/from instance | Implemented |
| `update` | Update instance configuration | Implemented |

### Enhanced Commands

| Command | Enhancement | Status |
|---------|-------------|--------|
| `list` | Added `-o json/yaml/table` output format | Implemented |
| `status` | Added `-o json/yaml/table` output format | Implemented |

### Output Formatting Infrastructure

New `pkg/output` package provides:
- `Formatter` type with JSON/YAML/table output
- `TableData` interface for tabular rendering
- Consistent `-o/--output` flag across commands

## Part 2: Global Verbosity Levels

### Verbosity Levels

| Level | Flag | Behavior |
|-------|------|----------|
| Quiet | `-q, --quiet` | Only errors |
| Normal | (default) | Info, Check, Warning, Error |
| Verbose | `-v, --verbose` | Normal + debug details |
| Debug | `-d, --debug` | Verbose + internal traces |

### Method Visibility Matrix

| Method | Quiet | Normal | Verbose | Debug |
|--------|-------|--------|---------|-------|
| Error | ✓ | ✓ | ✓ | ✓ |
| Warning | ✗ | ✓ | ✓ | ✓ |
| Info | ✗ | ✓ | ✓ | ✓ |
| Check | ✗ | ✓ | ✓ | ✓ |
| Debug | ✗ | ✗ | ✓ | ✓ |
| Trace | ✗ | ✗ | ✗ | ✓ |

### Flag Precedence

When multiple flags specified: `--debug` > `--verbose` > `--quiet`

### Logger Changes

```go
// internal/logger/logger.go

type Verbosity int

const (
    VerbosityQuiet Verbosity = iota
    VerbosityNormal
    VerbosityVerbose
    VerbosityDebug
)

type FunLogger struct {
    // ... existing fields ...
    Verbosity Verbosity
}

func (l *FunLogger) SetVerbosity(v Verbosity)
func (l *FunLogger) Debug(format string, a ...any)
func (l *FunLogger) Trace(format string, a ...any)
```

### CLI Integration

```go
// cmd/cli/main.go

c.Flags = []cli.Flag{
    &cli.BoolFlag{
        Name:    "quiet",
        Aliases: []string{"q"},
        Usage:   "Suppress non-error output",
    },
    &cli.BoolFlag{
        Name:    "verbose",
        Aliases: []string{"v"},
        Usage:   "Enable verbose output",
    },
    &cli.BoolFlag{
        Name:    "debug",
        Aliases: []string{"d"},
        Usage:   "Enable debug output",
        EnvVars: []string{"DEBUG"},
    },
}

c.Before = func(ctx *cli.Context) error {
    switch {
    case ctx.Bool("debug"):
        log.SetVerbosity(logger.VerbosityDebug)
    case ctx.Bool("verbose"):
        log.SetVerbosity(logger.VerbosityVerbose)
    case ctx.Bool("quiet"):
        log.SetVerbosity(logger.VerbosityQuiet)
    default:
        log.SetVerbosity(logger.VerbosityNormal)
    }
    return nil
}
```

### List Command Change

Rename `--quiet` to `--ids-only` to avoid conflict with global `--quiet`:

```bash
# Before
holodeck list --quiet

# After
holodeck list --ids-only
```

## Implementation Tasks

### Phase 1: Verbosity Infrastructure
1. [ ] Update `internal/logger/logger.go` with Verbosity type and methods
2. [ ] Add unit tests for `internal/logger/logger_test.go`
3. [ ] Update `cmd/cli/main.go` with global flags and Before hook
4. [ ] Update `cmd/cli/list/list.go` to rename `--quiet` to `--ids-only`

### Phase 2: Unit Tests for New Commands
5. [ ] Add tests for `pkg/output/output_test.go`
6. [ ] Add tests for `cmd/cli/describe/describe_test.go`
7. [ ] Add tests for `cmd/cli/get/get_test.go`
8. [ ] Add tests for `cmd/cli/ssh/ssh_test.go`
9. [ ] Add tests for `cmd/cli/scp/scp_test.go`
10. [ ] Add tests for `cmd/cli/update/update_test.go`

### Phase 3: Integration Tests
11. [ ] Add integration tests for SSH/SCP commands (mock SSH server)
12. [ ] Add integration tests for describe/get commands (mock instance data)

### Phase 4: Debug/Trace Instrumentation
13. [ ] Add Debug/Trace calls to `pkg/provider/aws/aws.go`
14. [ ] Add Debug/Trace calls to `pkg/provisioner/provisioner.go`

## Usage Examples

### Verbosity Levels

```bash
# Quiet - scripting friendly
$ holodeck -q create -f env.yaml && echo "done"
done

# Normal (default)
$ holodeck create -f env.yaml
✔ Created instance abc123
✔ Provisioning complete

# Verbose - operational details
$ holodeck -v create -f env.yaml
Creating AWS resources in us-west-2...
  VPC: vpc-12345
  Subnet: subnet-67890
✔ Created instance abc123
Running provisioning...
  Installing NVIDIA driver
✔ Provisioning complete

# Debug - troubleshooting
$ holodeck -d create -f env.yaml
[DEBUG] Loading environment from env.yaml
[DEBUG] AWS client initialized
[TRACE] CreateVPC: {...}
...
```

### New Commands

```bash
# Describe instance
$ holodeck describe abc123
=== Instance Information ===
ID:           abc123
Name:         gpu-test
...

# Get kubeconfig
$ holodeck get kubeconfig abc123
Kubeconfig saved to: ~/.kube/config-abc123

# SSH into instance
$ holodeck ssh abc123
ubuntu@ip-10-0-1-5:~$

# Run command via SSH
$ holodeck ssh abc123 -- nvidia-smi
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 560.35.03    Driver Version: 560.35.03    CUDA Version: 12.6     |
...

# Copy files
$ holodeck scp ./script.sh abc123:/tmp/
Copied ./script.sh -> /tmp/script.sh (1024 bytes)
```

## Testing Strategy

### Unit Tests
- Test logger verbosity filtering
- Test output formatter (JSON/YAML/table)
- Test command argument parsing
- Test path parsing for scp (local vs remote)

### Integration Tests
- Use mock SSH server for ssh/scp tests
- Use fixture cache files for describe/get/status tests
- Test error conditions (instance not found, SSH failures)

## Acceptance Criteria

- [ ] All commands support global `-q`, `-v`, `-d` flags
- [ ] `list` command uses `--ids-only` instead of `--quiet`
- [ ] All new commands have unit tests with >80% coverage
- [ ] Integration tests pass for SSH/SCP with mock server
- [ ] `go vet` and `go build` pass
- [ ] Documentation updated with new command examples
