# Operating System Selection

Holodeck supports automatic AMI resolution based on OS identifiers. Instead of
finding and specifying exact AMI IDs for each region, you can use the `os`
field to declare the operating system you want.

## Quick Start

```yaml
spec:
  instance:
    type: g4dn.xlarge
    region: us-west-2
    os: ubuntu-22.04  # AMI auto-resolved, username auto-detected
```

## Benefits

- **No AMI lookup required**: Holodeck resolves the correct AMI for your region
- **Auto-detected SSH username**: No need to specify `auth.username`
- **Always current**: Uses AWS SSM Parameter Store for latest AMI IDs
- **Multi-architecture support**: Works with both x86_64 and arm64 instances

## Supported Operating Systems

Run `holodeck os list` to see all available operating systems:

| OS ID | Family | SSH User | Package Manager | Architectures |
|-------|--------|----------|-----------------|---------------|
| ubuntu-24.04 | debian | ubuntu | apt | x86_64, arm64 |
| ubuntu-22.04 | debian | ubuntu | apt | x86_64, arm64 |
| ubuntu-20.04 | debian | ubuntu | apt | x86_64, arm64 |
| amazon-linux-2023 | amazon | ec2-user | dnf | x86_64, arm64 |
| rocky-9 | rhel | rocky | dnf | x86_64, arm64 |

### OS Family Support

- **Debian family** (Ubuntu, Debian): Full provisioning support for all components
- **RHEL family** (Rocky, Fedora, RHEL): Partial support - containerd works,
  other components being added
- **Amazon family** (Amazon Linux): Partial support - containerd works,
  other components being added

## CLI Commands

### List Available Operating Systems

```bash
holodeck os list
```

Output:
```
ID                   FAMILY    SSH USER    PACKAGE MGR  ARCHITECTURES
amazon-linux-2023    amazon    ec2-user    dnf          x86_64, arm64
rocky-9              rhel      rocky       dnf          x86_64, arm64
ubuntu-20.04         debian    ubuntu      apt          x86_64, arm64
ubuntu-22.04         debian    ubuntu      apt          x86_64, arm64
ubuntu-24.04         debian    ubuntu      apt          x86_64, arm64
```

### Get OS Details

```bash
holodeck os describe ubuntu-22.04
```

Output:
```
ID:              ubuntu-22.04
Name:            Ubuntu 22.04 LTS (Jammy Jellyfish)
Family:          debian
SSH Username:    ubuntu
Package Manager: apt
Min Root Volume: 20 GB
Architectures:   x86_64, arm64
AWS Owner ID:    099720109477
SSM Path:        /aws/service/canonical/ubuntu/server/22.04/stable/current/%s/hvm/ebs-gp3/ami-id
```

### Resolve AMI for a Region

```bash
# Get AMI ID for ubuntu-22.04 in us-west-2
holodeck os ami ubuntu-22.04 --region us-west-2

# Get ARM64 AMI
holodeck os ami ubuntu-22.04 --region us-west-2 --arch arm64
```

## Configuration Examples

### Simple Single-Node Environment

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: gpu-test
spec:
  provider: aws
  auth:
    keyName: my-key
    publicKey: ~/.ssh/id_rsa.pub
    privateKey: ~/.ssh/id_rsa
    # username auto-detected from OS
  instance:
    type: g4dn.xlarge
    region: us-west-2
    os: ubuntu-22.04
    image:
      architecture: x86_64
  containerRuntime:
    install: true
    name: containerd
  nvidiaDriver:
    install: true
  nvidiaContainerToolkit:
    install: true
```

### Multi-Node Cluster with Different OS per Node Pool

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: mixed-cluster
spec:
  provider: aws
  auth:
    keyName: my-key
    publicKey: ~/.ssh/id_rsa.pub
    privateKey: ~/.ssh/id_rsa
  cluster:
    region: us-west-2
    controlPlane:
      count: 3
      instanceType: m5.xlarge
      os: ubuntu-22.04
    workers:
      count: 2
      instanceType: g4dn.xlarge
      os: ubuntu-22.04
    highAvailability:
      enabled: true
  containerRuntime:
    install: true
    name: containerd
  kubernetes:
    install: true
```

### Explicit AMI Override

When you need to use a custom or private AMI, you can still specify it
explicitly. The explicit AMI takes precedence over the `os` field:

```yaml
spec:
  instance:
    type: g4dn.xlarge
    region: us-west-2
    # os: ubuntu-22.04  # Ignored when imageId is set
    image:
      imageId: ami-custom123456  # Custom AMI
      architecture: x86_64
  auth:
    username: myuser  # Required for custom AMIs
```

## AMI Resolution Process

Holodeck resolves AMIs using the following priority:

1. **Explicit ImageId**: If `image.imageId` is provided, it is used directly
2. **OS Field**: If `os` is specified, resolve via:
   - AWS SSM Parameter Store (fastest, always current)
   - EC2 DescribeImages API (fallback)
3. **Legacy Default**: Ubuntu 22.04 (backward compatibility)

### SSM Parameter Store

For supported operating systems, Holodeck uses AWS SSM Parameter Store to
get the latest official AMI ID. This ensures you always get the most current
image without needing to update your configuration.

Example SSM path for Ubuntu 22.04:
```
/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp3/ami-id
```

## Troubleshooting

### Unknown OS Error

```
Error: OS 'ubuntu-23.10' not found

Run 'holodeck os list' to see available operating systems
```

**Solution**: Use one of the supported OS identifiers listed in `holodeck os list`.

### Architecture Not Supported

```
Error: OS ubuntu-22.04 does not support architecture ppc64le (supported: x86_64, arm64)
```

**Solution**: Use a supported architecture (x86_64 or arm64).

### AMI Not Found in Region

```
Error: no images found for ubuntu-22.04 in region ap-northeast-3
```

**Solution**: The OS may not be available in all regions. Try a different
region or use an explicit AMI ID.

## Adding New Operating Systems

To request support for additional operating systems, please open an issue
at https://github.com/NVIDIA/holodeck/issues with:

- OS name and version
- AWS owner ID for the official AMIs
- SSM Parameter Store path (if available)
- Default SSH username
