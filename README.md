# Holodeck

> * Tech preview, under heavy development *

A tool for creating and managing GPU ready Cloud test environments.

## Installation

```bash
make build
mv ./bin/holodeck /usr/local/bin/holodeck
```

### Prerequisites

If utilizing the AWS provider, a valid AWS credentials must be available in the environment.

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: holodeck
  description: "Devel infra environment"
spec:
  provider: aws
```

If utilizing the SSH provider, a valid SSH key must and reachable host must be available in the environment file.

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: holodeck
  description: "Devel infra environment"
spec:
  provider: aws
  auth:
    keyName: eduardoa
    privateKey: "/Users/eduardoa/.ssh/eduardoa.pem"
  instance:
    hostUrl: "some-reachable-host-ip"
```

##  Usage

```bash
holodeck --help
```

### The Environment CRD

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: holodeck
  description: "Devel infra environment"
spec:
  provider: aws # or ssh for now
  auth:
    keyName: eduardoa
    privateKey: "/Users/eduardoa/.ssh/eduardoa.pem"
  instance: # if provider is ssh you need to define here the hostUrl
    type: g4dn.xlarge
    region: eu-north-1
    ingressIpRanges:
      - 192.168.1.0/26
    image:
      architecture: amd64
      imageId: ami-0fe8bec493a81c7da
  containerRuntime:
    install: true
    name: containerd
    version: 1.6.24
  kubernetes:
    install: true
    enable: true
    version: v1.28.5
```

### Create an environment

```bash
holodeck create -f ./examples/v1alpha1_environment.yaml
```

### Delete an environment

```bash
holodeck delete -f ./examples/v1alpha1_environment.yaml
```