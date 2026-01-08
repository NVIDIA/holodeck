# Cleanup Command

The `cleanup` command deletes AWS VPC resources, with optional GitHub job status
checking.

## Usage

```bash
holodeck cleanup [options] VPC_ID [VPC_ID...]
```

## Description

The cleanup command performs comprehensive deletion of AWS VPC resources including:

- EC2 instances
- Security groups (with ENI detachment)
- Subnets
- Route tables
- Internet gateways
- The VPC itself

Before deletion, it can optionally check GitHub Actions job status using VPC tags
to ensure jobs are completed.

## Options

- `--region, -r`: AWS region (overrides AWS_REGION environment variable)
- `--force, -f`: Force cleanup without checking GitHub job status

## Environment Variables

- `AWS_REGION`: Default AWS region if not specified via flag
- `AWS_DEFAULT_REGION`: Fallback region if AWS_REGION is not set
- `GITHUB_TOKEN`: GitHub token for checking job status (optional)

## Examples

### Clean up a single VPC

```bash
holodeck cleanup vpc-12345678
```

### Clean up multiple VPCs

```bash
holodeck cleanup vpc-12345678 vpc-87654321
```

### Force cleanup without job status check

```bash
holodeck cleanup --force vpc-12345678
```

### Clean up in a specific region

```bash
holodeck cleanup --region us-west-2 vpc-12345678
```

## GitHub Job Status Checking

If the VPC has the following tags and `GITHUB_TOKEN` is set:

- `GitHubRepository`: The repository in format `owner/repo`
- `GitHubRunId`: The GitHub Actions run ID

The command will check if all jobs in that run are completed before proceeding with
deletion. Use `--force` to skip this check.

## Notes

- The command handles dependencies between resources automatically
- Security groups attached to ENIs are detached before deletion
- Non-main route tables are handled appropriately
- VPC deletion includes retry logic (3 attempts with 30-second delays)
- Partial failures are logged but don't stop the cleanup process
