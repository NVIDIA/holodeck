# Status Command

The `status` command provides detailed information about a specific Holodeck
environment.

## Usage

```bash
holodeck status <instance-id> [flags]
```

## Flags

- `-c, --cachepath <dir>`  Path to the cache directory (optional)

## Examples

### Basic Status Check

```bash
holodeck status 123e4567-e89b-12d3-a456-426614174000
```

### Specify Cache Directory

```bash
holodeck status 123e4567-e89b-12d3-a456-426614174000 --cachepath=/tmp/holodeck-cache
```

## Sample Output

```text
Instance ID: 123e4567-e89b-12d3-a456-426614174000
Name: my-env
Provider: aws
Status: running
Created: 2024-06-01 12:00:00 (2h0m0s ago)
Cache File: /home/user/.cache/holodeck/123e4567-e89b-12d3-a456-426614174000.yaml
```

## Common Errors & Logs

- `instance ID is required` — You must provide an instance ID.
- `invalid instance ID` — The provided instance ID is not valid.
- `failed to get instance: ...` — The specified instance could not be found or
    loaded.

## Related Commands

- [create](create.md) - Create an environment
- [list](list.md) - List all environments
- [delete](delete.md) - Delete an environment
