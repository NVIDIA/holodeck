# Prerequisites

This document outlines the requirements and setup needed to use Holodeck
effectively.

## System Requirements

- Linux or macOS operating system (Windows is not supported)
- Go 1.20 or later
- Make
- Git

## Provider-Specific Requirements

### AWS Provider

To use the AWS provider, you need:

1. AWS CLI installed and configured
1. Valid AWS credentials in one of these locations:
  - `~/.aws/credentials`
  - Environment variables:
        - `AWS_ACCESS_KEY_ID`
        - `AWS_SECRET_ACCESS_KEY`
        - `AWS_SESSION_TOKEN` (if using temporary credentials)

1. Appropriate IAM permissions for:
  - EC2 instance management
  - VPC configuration
  - Security group management
  - IAM role management

### SSH Provider

To use the SSH provider, you need:

1. SSH key pair
1. Access to a reachable host
1. Proper network connectivity to the target host
1. Sufficient permissions on the target host

## Environment Configuration

### AWS Configuration Example

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: aws-env
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
```

### SSH Configuration Example

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: ssh-env
spec:
  provider: ssh
  auth:
    keyName: user
    privateKey: "/path/to/private/key"
  instance:
    hostUrl: "host.example.com"
```

## Network Requirements

- **Outbound Internet Access**: Required for package downloads and IP detection
- **IP Detection Services**: Access to public IP detection services (ipify.org, ifconfig.me, icanhazip.com, ident.me)
- **Security Group Rules**: Automatically configured for your detected public IP
- **VPC Configuration**: Automatically configured if using AWS provider

## GPU & Driver Requirements

- Compatible NVIDIA GPU (for GPU workloads)
- Supported NVIDIA driver version
  (see [Create Command documentation](../commands/create.md#supported-nvidia-driver-versions)
  for the list)
- CUDA toolkit (optional, only if your workloads require it)

For more information, see the [Quick Start Guide](quick-start.md)
or the [Command Reference](commands/).
