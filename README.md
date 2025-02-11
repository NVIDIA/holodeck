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
    keyName: user
    privateKey: "/Users/user/.ssh/user.pem"
  instance:
    hostUrl: "<some-reachable-host-ip>"
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
  provider: aws # or ssh currently supported
  auth:
    keyName: user
    privateKey: "/Users/user/.ssh/user.pem"
  instance: # if provider is ssh you need to define here the hostUrl
    type: g4dn.xlarge
    region: eu-north-1
    ingressIpRanges:
      - 192.168.1.0/26
    image:
      architecture: amd64
      imageId: ami-0fe8bec493a81c7da # Ubuntu 22.04 image
  containerRuntime:
    install: true
    name: containerd
    version: 1.6.24
  kubernetes:
    install: true
    installer: kubeadm # supported installers: kubeadm, kind, microk8s
    version: v1.28.5
```

The dependencies are resolved automatically, from top to bottom. Following the
pattern:

> Kubernetes -> Container Runtime -> Container Toolkit -> NVDriver

If Kubernetes is requested, and no container runtime is requested, a default
container runtime will be added to the environment..

If Container Toolkit is requested, and no container runtime is requested, a
default container runtime will be added to the environment.

### Create an environment

```bash
$ holodeck create -f ./examples/v1alpha1_environment.yaml
...
```

### Delete an environment

```bash
$ holodeck delete -f ./examples/v1alpha1_environment.yaml
...
```

### Dry Run

```bash
$ holodeck dryrun -f ./examples/v1alpha1_environment.yaml
Dryrun environment holodeck 🔍
✔       Checking if instance type g4dn.xlarge is supported in region eu-north-1
✔       Checking if image ami-0fe8bec493a81c7da is supported in region eu-north-1
✔      Resolving dependencies 📦
Dryrun succeeded 🎉
```

## Supported Cuda-Drivers

Supported Nvidia drivers are: 

```yaml
  nvidiaDriver:
    install: true
    version: <version>
```
Where `<version>` can be a prefix of any package version. The following are example package versions:

- 570.86.15-0ubuntu1
- 570.86.10-0ubuntu1
- 565.57.01-0ubuntu1
- 560.35.05-0ubuntu1
- 560.35.03-1
- 560.28.03-1
- 555.42.06-1
- 555.42.02-1
- 550.144.03-0ubuntu1
- 550.127.08-0ubuntu1
- 550.127.05-0ubuntu1
- 550.90.12-0ubuntu1
- 550.90.07-1
- 550.54.15-1
- 550.54.14-1
- 545.23.08-1
- 545.23.06-1
- 535.230.02-0ubuntu1
- 535.216.03-0ubuntu1
- 535.216.01-0ubuntu1
- 535.183.06-1
- 535.183.01-1
- 535.161.08-1
- 535.161.07-1
- 535.154.05-1
- 535.129.03-1
- 535.104.12-1
- 535.104.05-1
- 535.86.10-1
- 535.54.03-1
- 530.30.02-1
- 525.147.05-1
- 525.125.06-1
- 525.105.17-1
- 525.85.12-1
- 525.60.13-1
- 520.61.05-1
- 515.105.01-1
- 515.86.01-1
- 515.65.07-1
- 515.65.01-1
- 515.48.07-1
- 515.43.04-1
