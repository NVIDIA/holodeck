# Quick Start Guide

This guide will help you get started with Holodeck quickly.

## Installation

```bash
# Build the binary
make build

# Install to your system (requires sudo)
sudo mv ./bin/holodeck /usr/local/bin/holodeck
```

## Prerequisites

- Go 1.20+
- (For AWS) Valid AWS credentials in your environment
- (For SSH) Reachable host and valid SSH key

See [Prerequisites](prerequisites.md) for full details.

## Your First Environment

1. Create a basic environment configuration file (e.g., `environment.yaml`):

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-first-env
  description: "My first Holodeck environment"
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
    image:
      architecture: amd64
      imageId: ami-0fe8bec493a81c7da # Ubuntu 22.04 image
  kubernetes:
    install: true
    version: v1.28.5
```

1. Create the environment:

```bash
holodeck create -f environment.yaml
```

1. List environments and find your instance ID:

```bash
holodeck list
```

1. Check the status of your environment:

```bash
holodeck status <instance-id>
```

1. When done, delete the environment:

```bash
holodeck delete <instance-id>
```

## Next Steps

- Check out the [Prerequisites](prerequisites.md) for detailed setup
   requirements
- Explore the [Command Reference](commands/) for all available commands
- See [Examples](../examples/) for more complex configurations
