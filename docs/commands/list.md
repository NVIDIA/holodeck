# List Command

The `list` command displays all Holodeck environments and their current status.

## Usage

```bash
holodeck list [flags]
```

## Flags

- `-c, --cachepath <dir>`  Path to the cache directory (optional)
- `-q, --quiet`            Only display instance IDs (optional)

## Examples

### List All Environments

```bash
holodeck list
```

### List Only Instance IDs

```bash
holodeck list --quiet
```

### Specify Cache Directory

```bash
holodeck list --cachepath=/tmp/holodeck-cache
```

## Sample Output

```text
INSTANCE ID        NAME           PROVIDER   STATUS   PROVISIONED   CREATED              AGE
123e4567-...      my-env         aws        running  true          2024-06-01 12:00:00  2h
...
```

## Common Errors & Logs

- `No instances found` — No environments are currently managed by Holodeck.
- `failed to list instances: ...` — There was an error reading the cache or
    instance data.

## Related Commands

- [create](create.md) - Create an environment
- [status](status.md) - Check environment status
- [delete](delete.md) - Delete an environment
