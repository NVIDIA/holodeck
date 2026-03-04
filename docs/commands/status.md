# Status Command

The `status` command provides detailed information about a specific Holodeck
environment, including multinode cluster status and health checks.

## Usage

```bash
holodeck status <instance-id> [flags]
```

## Flags

- `-c, --cachepath <dir>`  Path to the cache directory (optional)
- `-l, --live`             Perform live health check via SSH (cluster only)

## Examples

### Basic Status Check

```bash
holodeck status 123e4567-e89b-12d3-a456-426614174000
```

### Specify Cache Directory

```bash
holodeck status 123e4567-e89b-12d3-a456-426614174000 --cachepath=/tmp/holodeck-cache
```

### Live Cluster Health Check

For multinode clusters, use the `--live` flag to connect via SSH and query
real-time Kubernetes cluster state:

```bash
holodeck status my-cluster --live
```

## Sample Output (Single Instance)

```text
Instance ID: 123e4567-e89b-12d3-a456-426614174000
Name: my-env
Provider: aws
Status: running
Created: 2024-06-01 12:00:00 (2h0m0s ago)
Cache File: /home/user/.cache/holodeck/123e4567-e89b-12d3-a456-426614174000.yaml
```

## Sample Output (Multinode Cluster)

```text
Environment: my-cluster
Provider: aws
Region: us-west-2

Cluster Status:
  Phase: Ready
  Total Nodes: 3
  Ready Nodes: 3
  Control Plane Endpoint: ec2-xx-xx-xx-xx.us-west-2.compute.amazonaws.com

Nodes:
  NAME                      ROLE           STATUS    PUBLIC IP       PRIVATE IP
  my-cluster-cp-0           control-plane  Ready     54.123.45.67    10.0.0.10
  my-cluster-worker-0       worker         Ready     54.123.45.68    10.0.0.11
  my-cluster-worker-1       worker         Ready     54.123.45.69    10.0.0.12
```

## Sample Output (Live Health Check)

```text
Cluster Health (Live):
  API Server: Running
  Total Nodes: 3
  Ready Nodes: 3
  Control Planes: 1
  Workers: 2

Node Details:
  NAME           STATUS   ROLES           VERSION   AGE
  ip-10-0-0-10   Ready    control-plane   v1.33.3   2h
  ip-10-0-0-11   Ready    worker          v1.33.3   2h
  ip-10-0-0-12   Ready    worker          v1.33.3   2h
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
