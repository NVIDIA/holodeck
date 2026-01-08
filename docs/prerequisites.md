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
- **IP Detection Services**: Access to public IP detection services
  (ipify.org, ifconfig.me, icanhazip.com, ident.me)
- **Security Group Rules**: Automatically configured for your detected public IP
- **VPC Configuration**: Automatically configured if using AWS provider

## GPU & Driver Requirements

- Compatible NVIDIA GPU (for GPU workloads)
- Supported NVIDIA driver version
  (see [Create Command documentation](../commands/create.md#supported-nvidia-driver-versions)
  for the list)
- CUDA toolkit (optional, only if your workloads require it)

### Kernel Compatibility

When installing NVIDIA drivers, Holodeck requires kernel headers that match your
running kernel version.

**Important Notes:**

- The NVIDIA driver needs to compile kernel modules, which requires exact kernel
  headers
- If exact kernel headers are not available, Holodeck will attempt to find
  compatible headers
- Using non-exact kernel headers may cause driver compilation issues

**Kernel Version Support:**

- Ubuntu 22.04: Kernels 5.15.x through 6.8.x (check repository availability)
- Ubuntu 24.04: Kernels 6.8.x and newer
- Custom kernels: Ensure corresponding headers are available in your package repositories

**Troubleshooting Kernel Header Issues:**

1. Check available kernel headers:

   ```bash
   apt-cache search linux-headers | grep $(uname -r)
   ```

1. Install specific kernel headers manually:

   ```bash
   sudo apt-get install linux-headers-$(uname -r)
   ```

1. If using a custom kernel, consider switching to a standard Ubuntu kernel

For more information, see the [Quick Start Guide](quick-start.md)
or the [Command Reference](commands/).
