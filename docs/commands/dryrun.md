# Dry Run Command

The `dryrun` command validates an environment configuration and simulates the
creation process without making any actual changes.

## Usage

```bash
holodeck dryrun -f <environment.yaml>
```

## Flags

- `-f, --envFile <file>`   Path to the environment YAML file (required)

## Examples

### Basic Dry Run

```bash
holodeck dryrun -f environment.yaml
```

## What Gets Validated

The dry run command checks:

- Configuration file syntax
- Provider credentials
- Resource availability
- Network configuration
- Component compatibility
- Dependencies resolution
- **Custom templates**: Validates template names, phases, source
    configuration, URL schemes (HTTPS required), checksum formats,
    environment variable names, and file path safety.

## Sample Output

```text
Dryrun environment my-environment 🔍
✔       Checking if instance type g4dn.xlarge is supported in region us-west-2
✔       Checking if image ami-0fe8bec493a81c7da is supported in region us-west-2
✔       Resolving dependencies 📦
Dryrun succeeded 🎉
```

## Common Errors & Logs

- `failed to read config file <file>: ...` — The environment YAML file is
    missing or invalid.
- `unknown provider ...` — The provider specified in the config is not
    supported.
- `failed to connect to <host>` — SSH connection failed (for SSH provider).
- `Dryrun succeeded 🎉` — All validations passed.

## Related Commands

- [create](create.md) - Create an environment
- [status](status.md) - Check environment status
- [list](list.md) - List all environments
