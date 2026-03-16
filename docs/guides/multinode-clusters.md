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

## Network Architecture

Cluster mode creates a production-grade VPC topology with public and private
subnets, a NAT gateway for outbound internet, and an NLB for API server access.

```text
┌──────────────────────────────── VPC 10.0.0.0/16 ────────────────────────────────┐
│                                                                                  │
│  ┌─── Public Subnet 10.0.1.0/24 ───┐    ┌─── Private Subnet 10.0.0.0/24 ───┐  │
│  │                                   │    │                                   │  │
│  │  NAT Gateway ──── Elastic IP      │    │  CP nodes (no public IP)          │  │
│  │  NLB ─────────── :6443            │    │  Worker nodes (no public IP)      │  │
│  │                                   │    │                                   │  │
│  │  Route: 0.0.0.0/0 → IGW          │    │  Route: 0.0.0.0/0 → NAT GW       │  │
│  └───────────────────────────────────┘    └───────────────────────────────────┘  │
│                                                                                  │
│  Internet Gateway                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

**Key properties:**

- All nodes run in the private subnet with no public IP addresses
- Outbound internet (for package installs, container pulls) goes through the NAT
  gateway
- The Kubernetes API server is exposed via an NLB in the public subnet on port
  6443
- SSH access to nodes uses AWS SSM port forwarding (no open SSH port to the
  internet)

### Single-Node vs Cluster Networking

| Feature | Single-node | Cluster |
|---------|-------------|---------|
| Subnets | 1 (public) | 2 (public + private) |
| Node public IPs | Yes | No |
| Internet route | IGW direct | NAT gateway |
| SSH access | Direct SSH | SSM port forwarding |
| API server | Direct to instance | NLB |
| Security groups | 1 shared | CP + Worker (least-privilege) |

## Security Groups

Cluster mode creates separate security groups for control-plane and worker nodes,
enforcing least-privilege access.

### Control-Plane Security Group

| Port | Protocol | Source | Purpose |
|------|----------|--------|---------|
| 22 | TCP | 0.0.0.0/0 | SSH (via SSM) |
| 6443 | TCP | NLB subnet CIDR + Worker SG | Kubernetes API |
| 2379-2380 | TCP | CP SG (self) | etcd peer/client |
| 10250 | TCP | CP SG (self) | kubelet |
| 10259 | TCP | CP SG (self) | kube-scheduler |
| 10257 | TCP | CP SG (self) | kube-controller-manager |
| 4789 | UDP | CP SG + Worker SG | Calico VXLAN |
| 4240 | TCP | CP SG + Worker SG | Calico Typha |

### Worker Security Group

| Port | Protocol | Source | Purpose |
|------|----------|--------|---------|
| 22 | TCP | 0.0.0.0/0 | SSH (via SSM) |
| 10250 | TCP | CP SG | kubelet (from control-plane only) |
| 30000-32767 | TCP | 0.0.0.0/0 | NodePort services |
| 4789 | UDP | CP SG + Worker SG | Calico VXLAN |
| 4240 | TCP | CP SG + Worker SG | Calico Typha |

## SSH Access via SSM

Since cluster nodes have no public IPs, Holodeck uses AWS Systems Manager (SSM)
port forwarding for SSH access during provisioning. This is handled automatically.

To manually SSH to a node after creation:

```bash
# Get node details
holodeck status <id>

# SSH via SSM port forwarding
aws ssm start-session \
  --target <instance-id> \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["22"],"localPortNumber":["2222"]}' &

ssh -i key.pem -p 2222 ubuntu@localhost
```

**Prerequisites for SSM:**

- The EC2 instances must have an IAM instance profile with the
  `AmazonSSMManagedInstanceCore` policy
- The SSM agent must be installed on the AMI (pre-installed on Ubuntu 22.04+,
  Amazon Linux 2023, and Rocky Linux 9)
- The AWS CLI v2 and Session Manager plugin must be installed on the client

## Calico Networking

- **Pod Network CIDR**: 192.168.0.0/16
- Source/Destination Check is automatically disabled on all EC2 network
  interfaces to allow Calico VXLAN encapsulation

## Troubleshooting

### Nodes not joining

1. Check security group rules allow inter-node communication (see tables above)
1. Verify the NAT gateway is active — nodes need outbound internet to pull
   container images and join the cluster
1. Check that the NLB health check on port 6443 is healthy

### SSH to debug

```bash
# Get node instance IDs
holodeck status <id>

# SSH via SSM (automatic in holodeck, manual below)
aws ssm start-session --target <instance-id>

# Once connected, check nodes
sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes
```

### View provisioning logs

```bash
# SSH to a node via SSM, then:
sudo journalctl -u kubelet -f
sudo kubeadm token list
```

### NAT gateway issues

If nodes cannot pull images or reach the internet:

```bash
# Check the NAT gateway state in AWS console
# Verify the private route table has 0.0.0.0/0 → NAT GW
# Check that the NAT gateway's Elastic IP is allocated
```
