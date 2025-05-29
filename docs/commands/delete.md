# Delete Command

The `delete` command removes a Holodeck environment and cleans up associated
resources.

## Usage

```bash
holodeck delete <instance-id> [flags]
```

## Flags

- `-c, --cachepath <dir>`  Path to the cache directory (optional)

## Examples

### Basic Deletion

```bash
holodeck delete 123e4567-e89b-12d3-a456-426614174000
```

### Specify Cache Directory

```bash
holodeck delete 123e4567-e89b-12d3-a456-426614174000 --cachepath=/tmp/holodeck-cache
```

## What Gets Deleted

- Cloud instances (if using AWS provider)
- Associated network resources
- Security groups
- IAM roles (if created)
- Local environment state

## Sample Output

```text
Successfully deleted instance 123e4567-e89b-12d3-a456-426614174000 (my-env)
```

## Common Errors & Logs

- `at least one instance ID is required` — You must provide an instance ID to delete.
- `failed to get instance <id>: ...` — The specified instance ID does not exist
    or cannot be found.
- `failed to delete instance <id>: ...` — There was an error during deletion.
- `Successfully deleted instance <id> (<name>)` — Success log after deletion.

## Related Commands

- [create](create.md) - Create an environment
- [status](status.md) - Check environment status
- [list](list.md) - List all environments
