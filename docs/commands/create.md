# Create Command

The `create` command creates a new Holodeck environment from a configuration
file.

## Usage

```bash
holodeck create [flags]
```

## Flags

- `-f, --envFile <file>`   Path to the environment YAML file (required)
- `-p, --provision`        Provision the environment after creation (optional)
- `-k, --kubeconfig <file>` Path to the kubeconfig file (optional)
- `-c, --cachepath <dir>`  Path to the cache directory (optional)

## Examples

### Basic Creation

```bash
holodeck create -f environment.yaml
```

### Create and Provision

```bash
holodeck create -f environment.yaml --provision
```

### Specify Kubeconfig and Cache Directory

```bash
holodeck create -f environment.yaml --kubeconfig=mykubeconfig --cachepath=/tmp/holodeck-cache
```

## Configuration File Format

The environment configuration file should be in YAML format:

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-environment
  description: "My test environment"
spec:
  provider: aws  # or ssh
  instance:
    type: g4dn.xlarge
    region: us-west-2
  kubernetes:
    install: true
    version: v1.28.5
```

## Sample Output

```text
Created instance 123e4567-e89b-12d3-a456-426614174000
```

## Common Errors & Logs

- `error reading config file: ...` — The environment YAML file is missing or
  invalid.
- `failed to provision: ...` — Provisioning failed due to a configuration or
  provider error.
- `Created instance <instance-id>` — Success log after creation.

## Supported NVIDIA Driver Versions

The following NVIDIA driver versions are supported (prefix match is allowed):

- 575.51.03-0ubuntu1
- 570.86.15-0ubuntu1
- 570.86.10-0ubuntu1
- 565.57.01-0ubuntu1
- 560.35.05-0ubuntu1
- 560.35.03-1
- 560.28.03-1
- 555.42.06-1
- 555.42.02-1
- 550.144.03-0ubuntu1
- 550.127.08-0ubuntu1
- 550.127.05-0ubuntu1
- 550.90.12-0ubuntu1
- 550.90.07-1
- 550.54.15-1
- 550.54.14-1
- 545.23.08-1
- 545.23.06-1
- 535.230.02-0ubuntu1
- 535.216.03-0ubuntu1
- 535.216.01-0ubuntu1
- 535.183.06-1
- 535.183.01-1
- 535.161.08-1
- 535.161.07-1
- 535.154.05-1
- 535.129.03-1
- 535.104.12-1
- 535.104.05-1
- 535.86.10-1
- 535.54.03-1
- 530.30.02-1
- 525.147.05-1
- 525.125.06-1
- 525.105.17-1
- 525.85.12-1
- 525.60.13-1
- 520.61.05-1
- 515.105.01-1
- 515.86.01-1
- 515.65.07-1
- 515.65.01-1
- 515.48.07-1
- 515.43.04-1

## Supported NVIDIA Driver Branches

The following NVIDIA driver branches are supported (prefix match is allowed):

- 575
- 570
- 565
- 560
- 555
- 550

## Related Commands

- [delete](delete.md) - Delete an environment
- [status](status.md) - Check environment status
- [dryrun](dryrun.md) - Test environment creation
