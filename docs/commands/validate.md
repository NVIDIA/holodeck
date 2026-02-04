# Validate Command

The `validate` command validates an environment file before creating an instance.
It checks the file structure, required fields, credentials, and SSH keys to help
catch configuration errors early.

## Usage

```bash
holodeck validate -f <env-file> [flags]
```

## Flags

- `-f, --envFile <file>`   Path to the Environment file (required)
- `--strict`                Fail on warnings (not just errors)

## What Gets Validated

The validate command performs the following checks:

### 1. Environment File
- File exists and is readable
- Valid YAML structure
- Proper Environment resource format

### 2. Required Fields
- Provider is specified (aws or ssh)
- Auth.KeyName is present
- Region is specified (for AWS provider)
- Instance type or cluster configuration is present
- HostUrl is specified (for SSH provider)

### 3. SSH Keys
- Private key file exists and is readable
- Public key file exists

### 4. AWS Credentials (AWS provider only)
- AWS credentials are configured
- Credentials are accessible

### 5. Component Configuration
- Component dependencies are satisfied (e.g., Container Toolkit requires container runtime)
- Kubernetes installer is valid (kubeadm, kind, or microk8s)
- NVIDIA Driver configuration is valid

## Examples

### Basic Validation

```bash
holodeck validate -f environment.yaml
```

### Strict Mode (Fail on Warnings)

```bash
holodeck validate -f environment.yaml --strict
```

## Sample Output

```
=== Validation Results ===

  ✓ Environment file
    Valid YAML structure
  ✓ Provider
    Provider: aws
  ✓ Auth.KeyName
    KeyName: my-key
  ✓ Region
    Region: us-west-2
  ✓ Instance.Type
    Instance type: g4dn.xlarge
  ✓ SSH private key
    Readable: /Users/me/.ssh/my-key.pem
  ✓ SSH public key
    Found: /Users/me/.ssh/my-key.pub
  ✓ AWS credentials
    Configured (source: SharedCredentialsProvider)
  ✗ Component dependencies
    Warning: Container Toolkit requires a container runtime

✅ Validation passed
```

## Exit Codes

- `0` - Validation passed (no errors, and no warnings in strict mode)
- `1` - Validation failed (errors found, or warnings found in strict mode)

## Common Errors & Logs

- `file not found: ...` — Environment file path is incorrect
- `invalid YAML: ...` — Environment file has YAML syntax errors
- `Provider is required` — Provider field is missing
- `KeyName is required` — Auth.KeyName is missing
- `Region is required for AWS provider` — Region not specified
- `Private key not found: ...` — SSH private key path is incorrect
- `Failed to load AWS config: ...` — AWS credentials are not configured
- `✅ Validation passed` — Success log when all checks pass

## Related Commands

- [create](create.md) - Create a new environment
- [provision](provision.md) - Provision an existing instance
- [dryrun](dryrun.md) - Test environment creation without creating resources
