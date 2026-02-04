# Provision Command

The `provision` command provisions or re-provisions an existing Holodeck instance.
Because templates are idempotent, it's safe to re-run provisioning to add components
or recover from failures.

## Usage

```bash
holodeck provision [instance-id] [flags]
```

## Modes

The provision command supports two modes:

1. **Instance mode**: Provision an existing instance by ID (default)
2. **SSH mode**: Provision a remote host directly without requiring an instance

## Flags

### Instance Mode Flags

- `-c, --cachepath <dir>`   Path to the cache directory (optional)
- `-k, --kubeconfig <file>`  Path to save the kubeconfig file (optional)

### SSH Mode Flags

- `--ssh`                    Enable SSH mode: provision a remote host directly
- `--host <address>`         Remote host address (required for SSH mode)
- `--key <path>`             Path to SSH private key (required for SSH mode)
- `-u, --user <username>`    SSH username (default: ubuntu)
- `-f, --envFile <file>`     Path to the Environment file (required for SSH mode)

## Examples

### Provision an Existing Instance

```bash
holodeck provision abc123
```

### Re-provision with Kubeconfig Download

```bash
holodeck provision abc123 -k ./kubeconfig
```

### SSH Mode: Provision a Remote Host Directly

```bash
holodeck provision --ssh --host 1.2.3.4 --key ~/.ssh/id_rsa -f env.yaml
```

### SSH Mode with Custom Username

```bash
holodeck provision --ssh --host myhost.example.com --key ~/.ssh/key --user ec2-user -f env.yaml
```

### SSH Mode with Kubeconfig Download

```bash
holodeck provision --ssh --host 1.2.3.4 --key ~/.ssh/id_rsa -f env.yaml -k ./kubeconfig
```

## How It Works

### Instance Mode

1. Retrieves instance details from the cache
2. Loads the environment configuration
3. Determines if provisioning a single node or cluster
4. Runs provisioning scripts on the instance(s)
5. Updates the provisioned status in the cache
6. Optionally downloads kubeconfig if Kubernetes is installed

### SSH Mode

1. Loads the environment file specified with `-f`
2. Overrides provider settings to use SSH
3. Connects to the remote host via SSH
4. Runs provisioning scripts directly on the host
5. Optionally downloads kubeconfig if Kubernetes is installed

## Supported Environments

- Single-node instances
- Multinode Kubernetes clusters
- AWS instances
- SSH-accessible hosts

## Common Errors & Logs

- `failed to get instance: ...` — Instance ID not found in cache
- `failed to read environment: ...` — Environment file is missing or invalid
- `failed to create provisioner: ...` — SSH connection failed
- `provisioning failed: ...` — Provisioning script execution failed
- `✅ Provisioning completed successfully` — Success log after completion

## Related Commands

- [create](create.md) - Create a new environment (with optional provisioning)
- [status](status.md) - Check environment status
- [validate](validate.md) - Validate environment file before provisioning
