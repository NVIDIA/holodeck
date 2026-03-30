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

Cluster mode creates a VPC (10.0.0.0/16) with two subnets. All cluster nodes —
both control-plane and workers — are placed in the **public subnet** with
public IP addresses for direct internet access via the Internet Gateway (IGW).
The private subnet (10.0.0.0/24) is created and reserved for future use (e.g.,
SSM VPC endpoints).

NAT Gateway is intentionally **not** provisioned by default. Each cluster node
gets a public IP and reaches the internet directly through the IGW. This avoids
consuming Elastic IP quota (AWS default: 5 per region), which can cause failures
when multiple holodeck environments run concurrently in CI.

```text
┌─────────────────────────────── VPC 10.0.0.0/16 ───────────────────────────────┐
│                                                                                │
│  ┌──── Public Subnet 10.0.1.0/24 ────────────────────────────────────────┐   │
│  │                                                                         │   │
│  │  CP node(s)     public IP ─── SSH / kubectl                            │   │
│  │  Worker node(s) public IP ─── SSH                                      │   │
│  │  NLB (HA only)  public DNS ── :6443 → CP instances                     │   │
│  │                                                                         │   │
│  │  Route: 0.0.0.0/0 → Internet Gateway                                   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                │
│  ┌──── Private Subnet 10.0.0.0/24 (reserved, no instances) ──────────────┐   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                │
│  Internet Gateway                                                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Key properties:**

- All cluster nodes run in the public subnet with public IP addresses
- Outbound internet access (package installs, image pulls) goes directly through
  the Internet Gateway — no NAT Gateway is needed
- In HA mode, the Kubernetes API server is exposed via an internet-facing NLB on
  port 6443; otherwise holodeck connects directly to the control-plane public IP
- Holodeck provisions nodes via direct SSH to the public IP; if a node has no
  public IP (e.g., future private-subnet deployment), holodeck automatically
  falls back to SSM port-forwarding transport
- Source/Destination Check is disabled on all network interfaces so Calico VXLAN
  encapsulation works correctly

### Single-Node vs Cluster Networking

| Feature | Single-node | Cluster |
|---------|-------------|---------|
| Subnets | 1 (public) | 2 (public + private reserved) |
| Node public IPs | Yes | Yes (public subnet) |
| Internet route | IGW direct | IGW direct |
| SSH access | Direct SSH | Direct SSH (SSM fallback for private-IP nodes) |
| API server | Direct to instance | Direct (single CP) or NLB (HA) |
| Security groups | 1 shared | CP + Worker (least-privilege) |

## Security Groups

Cluster mode creates separate security groups for control-plane and worker nodes,
enforcing least-privilege access. Security group sources reference other security
groups by ID rather than CIDR where possible, so rules remain tight as instances
scale.

### Control-Plane Security Group (`<name>-cp`)

| Port | Protocol | Source | Purpose |
|------|----------|--------|---------|
| 22 | TCP | Caller public IP | SSH access |
| 6443 | TCP | Caller IP + 10.0.1.0/24 (NLB subnet) + Worker SG | Kubernetes API |
| 2379-2380 | TCP | CP SG (self) | etcd peer/client |
| 10250 | TCP | CP SG (self) + Worker SG | kubelet |
| 10259 | TCP | CP SG (self) | kube-scheduler |
| 10257 | TCP | CP SG (self) | kube-controller-manager |
| 4789 | UDP | CP SG (self) + Worker SG | Calico VXLAN |
| 179 | TCP | CP SG (self) + Worker SG | Calico BGP |
| 5473 | TCP | CP SG (self) + Worker SG | Calico Typha |
| ICMP | - | 10.0.0.0/16 (VPC) | Ping / path MTU discovery |

### Worker Security Group (`<name>-worker`)

| Port | Protocol | Source | Purpose |
|------|----------|--------|---------|
| 22 | TCP | Caller public IP | SSH access |
| 10250 | TCP | CP SG | kubelet (from control-plane only) |
| 30000-32767 | TCP/UDP | 0.0.0.0/0 | NodePort services |
| 4789 | UDP | CP SG + Worker SG | Calico VXLAN |
| 179 | TCP | CP SG + Worker SG | Calico BGP |
| 5473 | TCP | CP SG + Worker SG | Calico Typha |
| ICMP | - | 10.0.0.0/16 (VPC) | Ping / path MTU discovery |

The control-plane security group is created first. After the worker security
group is created, cross-references are added back to the CP group for ports that
workers must reach (K8s API, kubelet, Calico overlay ports).

## SSH Transport

Holodeck provisions cluster nodes over SSH. The transport used depends on whether
the node has a public IP:

- **Direct SSH** (default for public-subnet nodes): holodeck dials the node's
  public IP on port 22 directly.
- **SSM port-forwarding** (automatic fallback for private-subnet nodes): if a
  node has no public IP but has an EC2 instance ID, holodeck uses
  `aws ssm start-session` with `AWS-StartPortForwardingSession` to tunnel SSH
  through AWS Systems Manager. No bastion host or open inbound SSH port is
  required.

Since all nodes currently use the public subnet, direct SSH is the normal path.
The SSM fallback is wired and ready for deployments that move nodes to private
subnets.

### Manual SSM Access (for private-subnet nodes)

```bash
# Start an SSM port-forwarding session to the node's SSH port
aws ssm start-session \
  --target <instance-id> \
  --region <region> \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["22"],"localPortNumber":["2222"]}' &

# Connect through the local tunnel
ssh -i key.pem -p 2222 ubuntu@localhost
```

**Prerequisites for SSM transport:**

- EC2 instances need an IAM instance profile with the
  `AmazonSSMManagedInstanceCore` policy
- The SSM agent must be installed on the AMI (pre-installed on Ubuntu 22.04+,
  Amazon Linux 2023, and Rocky Linux 9)
- AWS CLI v2 and the Session Manager plugin must be installed on the client

## Network Load Balancer (HA Mode)

When `highAvailability.enabled: true`, holodeck creates an internet-facing
Network Load Balancer in the public subnet:

- **Scheme**: internet-facing (IPv4)
- **Listener**: TCP port 6443
- **Target group**: all control-plane instances, port 6443 (TCP health check,
  10-second interval, 2-of-2 threshold)
- **Name pattern**: `<environment-name>-nlb` (truncated to 32 characters)
- **Endpoint**: `Status.Cluster.LoadBalancerDNS` in the environment status

kubeadm is configured with the NLB DNS name as the control-plane endpoint so
that worker joins and client kubeconfigs always use the load-balanced address,
surviving individual control-plane node restarts.

## Calico Networking

- **Pod Network CIDR**: 192.168.0.0/16
- Source/Destination Check is automatically disabled on all EC2 network
  interfaces to allow Calico VXLAN encapsulation

## Troubleshooting

### Nodes not joining

1. Check security group rules allow inter-node communication (see tables above)
1. Verify internet connectivity — nodes need outbound access to pull container
   images and reach the package repositories
1. If HA is enabled, check that the NLB health check on port 6443 is healthy

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

### Nodes cannot reach the internet

Cluster nodes use the public subnet with direct IGW access. If they cannot pull
images or reach package repositories:

```bash
# Confirm the public route table has 0.0.0.0/0 → Internet Gateway
# Verify the node's public IP is assigned (visible in holodeck status)
# Check the security group allows outbound traffic (default AWS SGs allow all egress)
```
