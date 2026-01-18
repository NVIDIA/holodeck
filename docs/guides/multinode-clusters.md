# Multinode Kubernetes Clusters

Holodeck supports creating multinode Kubernetes clusters on AWS with
configurable control plane and worker node pools.

## Non-Interactive Mode

When running Holodeck in scripts or CI/CD pipelines, use non-interactive mode
to skip all prompts:

```bash
# Using the CLI flag
holodeck create -f cluster.yaml --provision --non-interactive

# Using environment variable
export HOLODECK_NONINTERACTIVE=true
holodeck create -f cluster.yaml --provision

# CI environments are automatically detected
# (CI=true is set by most CI systems)
```

**Note:** For clusters with 5+ nodes, Holodeck will prompt (in interactive mode)
whether you want dedicated control-plane nodes. In non-interactive mode, the
value from your YAML file is used as-is.

## Overview

Instead of the single-node `instance` configuration, use the `cluster` block to
define a multinode deployment:

```yaml
spec:
  cluster:
    region: us-west-2
    controlPlane:
      count: 1
      instanceType: m5.xlarge
    workers:
      count: 2
      instanceType: g4dn.xlarge
```

## Quick Start

### Simple Cluster (1 CP + 2 Workers)

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-cluster
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
    workers:
      count: 2
      instanceType: g4dn.xlarge

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

Create and provision:

```bash
holodeck create -f cluster.yaml --provision -k kubeconfig.yaml
```

## Configuration Reference

### Cluster Spec

| Field | Type | Description |
|-------|------|-------------|
| `region` | string | AWS region for all nodes (required) |
| `controlPlane` | ControlPlaneSpec | Control plane node configuration |
| `workers` | WorkerPoolSpec | Worker node pool configuration |
| `highAvailability` | HAConfig | HA settings (optional) |

### Control Plane Spec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `count` | int32 | 1 | Number of control plane nodes (1, 3, 5, or 7) |
| `instanceType` | string | m5.xlarge | EC2 instance type |
| `dedicated` | bool | false | Keep NoSchedule taint (no workloads) |
| `labels` | map | - | Custom Kubernetes labels |
| `rootVolumeSizeGB` | int32 | 64 | Root volume size in GB |

### Worker Pool Spec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `count` | int32 | 1 | Number of worker nodes |
| `instanceType` | string | g4dn.xlarge | EC2 instance type |
| `labels` | map | - | Custom Kubernetes labels |
| `rootVolumeSizeGB` | int32 | 64 | Root volume size in GB |

### High Availability Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable HA mode |
| `etcdTopology` | string | stacked | `stacked` or `external` |
| `loadBalancerType` | string | nlb | `nlb` or `alb` |

## High Availability Clusters

For production workloads, use an HA configuration with 3+ control plane nodes:

```yaml
cluster:
  region: us-west-2
  controlPlane:
    count: 3
    instanceType: m5.xlarge
    dedicated: true  # No workloads on control plane
  workers:
    count: 3
    instanceType: g4dn.xlarge
  highAvailability:
    enabled: true
    etcdTopology: stacked
    loadBalancerType: nlb
```

### Why 3 Control Plane Nodes

- **etcd quorum**: Requires majority for writes (2 of 3)
- **Fault tolerance**: Survives 1 node failure
- **Odd numbers**: Always use 1, 3, 5, or 7 for proper quorum

## Node Labels and Taints

### Default Labels

All nodes receive:

- `nvidia.com/holodeck.managed=true`
- `nvidia.com/holodeck.role=control-plane` or `worker`

Workers also receive:

- `node-role.kubernetes.io/worker=`

### Custom Labels

```yaml
controlPlane:
  labels:
    environment: production
    tier: control
workers:
  labels:
    environment: production
    nvidia.com/gpu.present: "true"
```

### Control Plane Scheduling

By default, workloads can be scheduled on control plane nodes. To prevent this:

```yaml
controlPlane:
  dedicated: true  # Keeps NoSchedule taint
```

## Monitoring Cluster Status

### Cached Status

```bash
holodeck status <instance-id>
```

Shows:

- Cluster configuration
- Node list with IPs
- Control plane endpoint

### Live Health Check

```bash
holodeck status <instance-id> --live
```

Connects via SSH to query real Kubernetes state:

- API server status
- Node ready state
- Per-node Kubernetes version

## Examples

Example files are available in `examples/`:

- `aws_cluster_simple.yaml` - Simple 1+2 cluster
- `aws_cluster_ha.yaml` - HA cluster with 3 CPs
- `aws_cluster_minimal.yaml` - Minimal cluster without GPU

## Networking

Holodeck configures:

- **VPC**: 10.0.0.0/16
- **Subnet**: 10.0.0.0/24
- **Pod Network**: 192.168.0.0/16 (Calico)
- **Security Groups**: SSH, K8s API, inter-node communication

For Calico networking, Source/Destination Check is automatically disabled
on all EC2 network interfaces.

## Troubleshooting

### Nodes not joining

Check that all security group rules are in place:

- Port 6443 (API server)
- Port 10250 (kubelet)
- Port 2379-2380 (etcd)
- Port 4789/udp (Calico VXLAN)

### SSH to debug

```bash
# Get node IPs
holodeck status <id>

# SSH to control plane
ssh -i key.pem ubuntu@<control-plane-ip>

# Check nodes
sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes
```

### View provisioning logs

During creation, logs stream to stdout. For post-creation debugging:

```bash
# SSH and check kubelet
sudo journalctl -u kubelet -f

# Check kubeadm
sudo kubeadm token list
```
