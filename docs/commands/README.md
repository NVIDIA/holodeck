# Command Reference

This document provides detailed information about all available Holodeck
commands.

## Basic Commands

- [create](create.md) - Create a new environment
- [delete](delete.md) - Delete an existing environment
- [list](list.md) - List all environments
- [status](status.md) - Check the status of an environment
- [dryrun](dryrun.md) - Perform a dry run of environment creation

## Command Usage

All commands follow this general pattern:

```bash
holodeck [command] [flags]
```

For detailed help on any command:

```bash
holodeck [command] --help
```

## Global Options

- `--log-level string` - Log level (debug, info, warn, error) (default: "info")
- `--no-color` - Disable color output

## Examples

### Create an Environment

```bash
holodeck create -f environment.yaml
```

### List Environments

```bash
holodeck list
```

### Check Environment Status

```bash
holodeck status <instance-id>
```

### Delete an Environment

```bash
holodeck delete <instance-id>
```

### Dry Run

```bash
holodeck dryrun -f environment.yaml
```

For detailed information about each command, click on the command name above.
