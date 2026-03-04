# Holodeck Example Configurations

This directory provides example environment configuration files for Holodeck.
Use these as starting points for your own environments.

## Available Examples

### 1. Basic AWS Environment (Kubeadm)

**File:** [`examples/aws_kubeadm.yaml`](../../examples/aws_kubeadm.yaml)

A minimal AWS environment using the kubeadm installer for Kubernetes.

```bash
holodeck create -f examples/aws_kubeadm.yaml
```

### 2. Basic AWS Environment (Kind)

**File:** [`examples/aws_kind.yaml`](../../examples/aws_kind.yaml)

A minimal AWS environment using the kind installer for Kubernetes.

```bash
holodeck create -f examples/aws_kind.yaml
```

### 3. Generic v1alpha1 Environment

**File:**
[`examples/v1alpha1_environment.yaml`](../../examples/v1alpha1_environment.yaml)

A generic example showing the full v1alpha1 environment spec, including
provider, instance, and Kubernetes options.

```bash
holodeck create -f examples/v1alpha1_environment.yaml
```

### 4. Custom Kubeadm Config

**File:** [`examples/kubeadm-config.yaml`](../../examples/kubeadm-config.yaml)

A sample kubeadm configuration file for advanced Kubernetes cluster setup.
Use with the `kubeadm` installer.

### 5. Kind Cluster Config

**File:** [`examples/kind.yaml`](../../examples/kind.yaml)

A sample kind cluster configuration for use with the kind installer.

---

## Multinode Cluster Examples

### 6. Simple Cluster (1 CP + 2 Workers)

**File:**
[`examples/aws_cluster_simple.yaml`](../../examples/aws_cluster_simple.yaml)

A basic multinode Kubernetes cluster with 1 control plane and 2 GPU worker
nodes.

```bash
holodeck create -f examples/aws_cluster_simple.yaml --provision
```

### 7. HA Cluster (3 CP + Workers)

**File:** [`examples/aws_cluster_ha.yaml`](../../examples/aws_cluster_ha.yaml)

A high-availability cluster with 3 control plane nodes for production workloads.

```bash
holodeck create -f examples/aws_cluster_ha.yaml --provision
```

### 8. Minimal Cluster (No GPU)

**File:**
[`examples/aws_cluster_minimal.yaml`](../../examples/aws_cluster_minimal.yaml)

A minimal cluster configuration for testing without GPU instances.

```bash
holodeck create -f examples/aws_cluster_minimal.yaml --provision
```

See the [Multinode Clusters Guide](../guides/multinode-clusters.md) for detailed
configuration options.

---

## Updated AWS Examples

The example configurations now show that `ingressIpRanges` is optional:

**File:** [`examples/aws_kubeadm.yaml`](../../examples/aws_kubeadm.yaml)

```yaml
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: us-west-2
    # ingressIpRanges is now optional - your IP is detected automatically
    image:
      architecture: amd64
```

**File:** [`examples/aws_kind.yaml`](../../examples/aws_kind.yaml)

```yaml
spec:
  provider: aws
  instance:
    type: g4dn.xlarge
    region: eu-north-1
    # ingressIpRanges is now optional - your IP is detected automatically
    image:
      architecture: amd64
```

### Benefits of Automated IP Detection

- **Simplified Configuration**: No need to manually find and specify your
  public IP
- **Dynamic IP Support**: Works with changing IP addresses (DHCP, mobile
  networks)
- **Reduced Errors**: Eliminates "CIDR block malformed" errors
- **Better Security**: Ensures only your current public IP has access

## How to Use These Examples

1. Copy the desired YAML file to your working directory (optional).
1. Edit the file as needed (e.g., update region, instance type, image ID).
1. Create the environment:

   ```bash
   holodeck create -f <your-config>.yaml
   ```

1. Use `holodeck list`, `holodeck status <instance-id>`,
   and `holodeck delete <instance-id>` to manage your environment.

---

For more details on configuration options, see the
[Command Reference](../commands/README.md) and
[Quick Start Guide](../quick-start.md).
