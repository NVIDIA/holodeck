# RPM Distribution Guide

This guide covers using Holodeck with RPM-based Linux distributions: Rocky
Linux 9 and Amazon Linux 2023.

## Supported Distributions

| Distribution | OS ID | SSH User | Container Runtimes | Kubernetes |
|---|---|---|---|---|
| Rocky Linux 9 | rocky-9 | rocky | Docker, containerd, CRI-O | kubeadm |
| Amazon Linux 2023 | amazon-linux-2023 | ec2-user | Docker, containerd, CRI-O | kubeadm |

## Quick Start

### Rocky Linux 9 — Single Node with GPU

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: rocky9-gpu
spec:
  provider: aws
  auth:
    keyName: my-key
    privateKey: ~/.ssh/my-key.pem
  instance:
    type: g4dn.xlarge
    region: us-west-2
    os: rocky-9
    image:
      architecture: amd64
  containerRuntime:
    install: true
    name: containerd
  nvidiaDriver:
    install: true
  nvidiaContainerToolkit:
    install: true
  kubernetes:
    install: true
    installer: kubeadm
```

### Amazon Linux 2023 — Single Node with GPU

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: al2023-gpu
spec:
  provider: aws
  auth:
    keyName: my-key
    privateKey: ~/.ssh/my-key.pem
  instance:
    type: g4dn.xlarge
    region: us-west-2
    os: amazon-linux-2023
    image:
      architecture: amd64
  containerRuntime:
    install: true
    name: docker
  nvidiaDriver:
    install: true
  nvidiaContainerToolkit:
    install: true
  kubernetes:
    install: true
    installer: kubeadm
```

### Rocky Linux 9 — Multi-Node Cluster

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: rocky9-cluster
spec:
  provider: aws
  auth:
    keyName: my-key
    privateKey: ~/.ssh/my-key.pem
  cluster:
    region: us-west-2
    controlPlane:
      count: 1
      instanceType: m5.xlarge
      os: rocky-9
      rootVolumeSizeGB: 100
    workers:
      count: 2
      instanceType: g4dn.xlarge
      os: rocky-9
      rootVolumeSizeGB: 100
  nvidiaDriver:
    install: true
  nvidiaContainerToolkit:
    install: true
  containerRuntime:
    install: true
    name: containerd
  kubernetes:
    install: true
    installer: kubeadm
```

## RPM-Specific Considerations

### Package Manager

RPM distributions use `dnf` as the package manager. Holodeck detects the OS
family automatically and uses the correct package manager — no manual
configuration is needed.

### Amazon Linux and Docker

Amazon Linux 2023 does not have native Docker packages. Holodeck automatically
uses Fedora-compatible Docker repository packages (mapped to Fedora 39), which
are tested and stable on AL2023.

### SELinux

Rocky Linux 9 ships with SELinux in enforcing mode by default. Holodeck's
provisioning templates handle SELinux contexts for container runtimes
automatically. If you encounter permission issues on custom configurations,
check SELinux status:

```bash
# Check current SELinux mode
getenforce

# View recent denials
sudo ausearch -m avc -ts recent
```

### Firewall

Amazon Linux 2023 has `firewalld` disabled by default. Rocky Linux 9 may have
`firewalld` enabled. Holodeck configures the necessary ports during
provisioning, but if you have custom firewall rules, ensure these ports are
open:

- **6443/tcp** — Kubernetes API server
- **10250/tcp** — kubelet
- **2379-2380/tcp** — etcd (control plane only)
- **4789/udp** — Calico VXLAN overlay

### Kernel Headers

NVIDIA driver compilation requires kernel headers matching the running kernel.
On RPM distributions, these are provided by the `kernel-devel` package:

```bash
# Check installed kernel headers
rpm -qa | grep kernel-devel

# Install matching headers
sudo dnf install -y kernel-devel-$(uname -r)
```

## ARM64 Support

Both Rocky Linux 9 and Amazon Linux 2023 support ARM64 (aarch64) instances.
Use the `arm64` architecture in your configuration:

```yaml
instance:
  type: g5g.xlarge  # Graviton GPU instance
  os: rocky-9
  image:
    architecture: arm64
```

## Troubleshooting

### Cloud-init Delays

RPM distributions may take longer for cloud-init to complete on first boot.
Holodeck waits for cloud-init automatically, but if you see timeouts, the
instance may need a larger instance type or more patience on initial
provisioning.

### Package Conflicts

If provisioning fails during package installation, SSH into the instance and
check for package conflicts:

```bash
# Check for broken packages
sudo dnf check

# View provisioning logs
sudo journalctl -u holodeck-provision --no-pager
```
