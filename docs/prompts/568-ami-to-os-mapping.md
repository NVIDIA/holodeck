# Issue #568: Internal AMI-to-OS Mapping for Simplified Configuration

## Overview

This document provides implementation guidance for issue #568 - creating an internal
mapping system that associates operating systems with their corresponding Amazon Machine
Images (AMIs) across regions.

## Problem Statement

Currently, users must:
1. Find the correct AMI ID for their desired OS and region manually
2. Ensure the AMI is available in their chosen region
3. Know which username to use for SSH (differs by OS)
4. Understand the architecture compatibility

**Current configuration (complex):**
```yaml
spec:
  instance:
    image:
      imageId: ami-0c55b159cbfafe1f0  # What OS is this?
      architecture: x86_64
  auth:
    username: ubuntu  # How do I know this?
```

**Desired configuration (simple):**
```yaml
spec:
  instance:
    os: ubuntu-22.04  # AMI auto-resolved based on region and architecture
```

---

## Research Summary

### 1. AWS AMI Owner IDs

| OS Family | Owner ID | Owner Name |
|-----------|----------|------------|
| Ubuntu | `099720109477` | Canonical |
| Ubuntu (Marketplace) | `679593333241` | AWS Marketplace |
| Amazon Linux | `amazon` | Amazon (use `--owners amazon`) |
| Rocky Linux | `792107900819` | Rocky Enterprise Software Foundation |
| Debian | `136693071363` | Debian |
| Fedora | `125523088429` | Fedora |

### 2. AMI Naming Conventions

| OS | Name Pattern | Example |
|----|--------------|---------|
| Ubuntu 22.04 | `ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-<arch>-server-*` | `ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20240101` |
| Ubuntu 24.04 | `ubuntu/images/hvm-ssd/ubuntu-noble-24.04-<arch>-server-*` | `ubuntu/images/hvm-ssd/ubuntu-noble-24.04-amd64-server-20240101` |
| Amazon Linux 2023 | `al2023-ami-*-kernel-*-<arch>` | `al2023-ami-2023.0.20240101-kernel-6.1-x86_64` |
| Rocky Linux 9 | `Rocky-9-EC2-Base-*.<arch>-*` | `Rocky-9-EC2-Base-9.3-20240101.0.x86_64` |

### 3. SSH Default Usernames

| OS | Default Username |
|----|-----------------|
| Ubuntu | `ubuntu` |
| Amazon Linux | `ec2-user` |
| Rocky Linux | `rocky` |
| Debian | `admin` |
| Fedora | `fedora` |
| RHEL | `ec2-user` |

### 4. AWS SSM Parameter Store Paths

AWS provides public SSM parameters for latest AMI IDs:

```bash
# Ubuntu (Canonical)
/aws/service/canonical/ubuntu/server/<version>/stable/current/<arch>/hvm/ebs-gp3/ami-id

# Examples:
/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id
/aws/service/canonical/ubuntu/server/22.04/stable/current/arm64/hvm/ebs-gp3/ami-id

# Amazon Linux 2023
/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-<arch>

# Examples:
/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64
/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64
```

### 5. Current Implementation Analysis

**File:** `pkg/provider/aws/image.go`

```go
// Current hardcoded approach
awsOwner := []string{"099720109477", "679593333241"}  // Ubuntu only

// Hardcoded to Ubuntu 22.04 Jammy
filterNameValue = append(filterNameValue, 
    fmt.Sprintf("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-20*", arch))
```

**Limitations:**
- Only supports Ubuntu 22.04
- No OS abstraction
- Username hardcoded elsewhere

---

## Implementation Plan

### Phase 1: Data Model Design

Create new package: `internal/ami/`

```go
// internal/ami/types.go
package ami

// OSFamily groups similar operating systems
type OSFamily string

const (
    OSFamilyDebian OSFamily = "debian"  // Ubuntu, Debian
    OSFamilyRHEL   OSFamily = "rhel"    // Rocky, Fedora, RHEL, CentOS
    OSFamilyAmazon OSFamily = "amazon"  // Amazon Linux
)

// PackageManager indicates the package management system
type PackageManager string

const (
    PackageManagerAPT PackageManager = "apt"
    PackageManagerDNF PackageManager = "dnf"
    PackageManagerYUM PackageManager = "yum"
)

// OSImage defines metadata for an operating system image
type OSImage struct {
    // ID is the short identifier (e.g., "ubuntu-22.04")
    ID string
    
    // Name is the display name (e.g., "Ubuntu 22.04 LTS (Jammy Jellyfish)")
    Name string
    
    // Family groups related OSes for template selection
    Family OSFamily
    
    // SSHUsername is the default SSH user
    SSHUsername string
    
    // PackageManager for template selection
    PackageManager PackageManager
    
    // MinRootVolumeGB is the minimum root volume size
    MinRootVolumeGB int32
    
    // OwnerID for DescribeImages filter
    OwnerID string
    
    // NamePattern for DescribeImages filter (with %s for architecture)
    NamePattern string
    
    // SSMPath for SSM Parameter Store lookup (with %s for architecture)
    // Empty if SSM not supported
    SSMPath string
    
    // Architectures supported (x86_64, arm64)
    Architectures []string
}

// ResolvedAMI contains the resolved AMI information
type ResolvedAMI struct {
    ImageID        string
    SSHUsername    string
    OSFamily       OSFamily
    PackageManager PackageManager
}
```

### Phase 2: OS Registry

```go
// internal/ami/registry.go
package ami

var registry = map[string]OSImage{
    "ubuntu-24.04": {
        ID:             "ubuntu-24.04",
        Name:           "Ubuntu 24.04 LTS (Noble Numbat)",
        Family:         OSFamilyDebian,
        SSHUsername:    "ubuntu",
        PackageManager: PackageManagerAPT,
        MinRootVolumeGB: 20,
        OwnerID:        "099720109477",
        NamePattern:    "ubuntu/images/hvm-ssd/ubuntu-noble-24.04-%s-server-*",
        SSMPath:        "/aws/service/canonical/ubuntu/server/24.04/stable/current/%s/hvm/ebs-gp3/ami-id",
        Architectures:  []string{"x86_64", "arm64"},
    },
    "ubuntu-22.04": {
        ID:             "ubuntu-22.04",
        Name:           "Ubuntu 22.04 LTS (Jammy Jellyfish)",
        Family:         OSFamilyDebian,
        SSHUsername:    "ubuntu",
        PackageManager: PackageManagerAPT,
        MinRootVolumeGB: 20,
        OwnerID:        "099720109477",
        NamePattern:    "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-*",
        SSMPath:        "/aws/service/canonical/ubuntu/server/22.04/stable/current/%s/hvm/ebs-gp3/ami-id",
        Architectures:  []string{"x86_64", "arm64"},
    },
    "amazon-linux-2023": {
        ID:             "amazon-linux-2023",
        Name:           "Amazon Linux 2023",
        Family:         OSFamilyAmazon,
        SSHUsername:    "ec2-user",
        PackageManager: PackageManagerDNF,
        MinRootVolumeGB: 20,
        OwnerID:        "amazon",
        NamePattern:    "al2023-ami-*-kernel-*-%s",
        SSMPath:        "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-%s",
        Architectures:  []string{"x86_64", "arm64"},
    },
    "rocky-9": {
        ID:             "rocky-9",
        Name:           "Rocky Linux 9",
        Family:         OSFamilyRHEL,
        SSHUsername:    "rocky",
        PackageManager: PackageManagerDNF,
        MinRootVolumeGB: 20,
        OwnerID:        "792107900819",
        NamePattern:    "Rocky-9-EC2-Base-*.%s-*",
        SSMPath:        "", // No SSM support
        Architectures:  []string{"x86_64", "arm64"},
    },
}

// Get returns an OSImage by ID
func Get(id string) (*OSImage, bool) {
    img, ok := registry[id]
    return &img, ok
}

// List returns all supported OS IDs
func List() []string {
    ids := make([]string, 0, len(registry))
    for id := range registry {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    return ids
}

// All returns all OSImage entries
func All() []OSImage {
    images := make([]OSImage, 0, len(registry))
    for _, img := range registry {
        images = append(images, img)
    }
    return images
}
```

### Phase 3: AMI Resolver

```go
// internal/ami/resolver.go
package ami

import (
    "context"
    "fmt"
    "sort"
    "strings"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    "github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Resolver resolves OS IDs to AMI IDs
type Resolver struct {
    ec2Client *ec2.Client
    ssmClient *ssm.Client
    region    string
}

// NewResolver creates a new AMI resolver
func NewResolver(ec2Client *ec2.Client, ssmClient *ssm.Client, region string) *Resolver {
    return &Resolver{
        ec2Client: ec2Client,
        ssmClient: ssmClient,
        region:    region,
    }
}

// Resolve looks up the AMI for the given OS, region, and architecture
func (r *Resolver) Resolve(ctx context.Context, osID, arch string) (*ResolvedAMI, error) {
    // Normalize architecture
    arch = normalizeArch(arch)
    
    // Get OS metadata
    osImage, ok := Get(osID)
    if !ok {
        return nil, fmt.Errorf("unknown OS: %s (run 'holodeck os list' for available options)", osID)
    }
    
    // Validate architecture
    if !contains(osImage.Architectures, arch) {
        return nil, fmt.Errorf("OS %s does not support architecture %s", osID, arch)
    }
    
    // Try SSM first if available (fastest, always up-to-date)
    if osImage.SSMPath != "" && r.ssmClient != nil {
        amiID, err := r.resolveViaSSM(ctx, osImage, arch)
        if err == nil {
            return &ResolvedAMI{
                ImageID:        amiID,
                SSHUsername:    osImage.SSHUsername,
                OSFamily:       osImage.Family,
                PackageManager: osImage.PackageManager,
            }, nil
        }
        // Fall through to DescribeImages on SSM failure
    }
    
    // Fall back to DescribeImages
    amiID, err := r.resolveViaDescribeImages(ctx, osImage, arch)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve AMI for %s: %w", osID, err)
    }
    
    return &ResolvedAMI{
        ImageID:        amiID,
        SSHUsername:    osImage.SSHUsername,
        OSFamily:       osImage.Family,
        PackageManager: osImage.PackageManager,
    }, nil
}

func (r *Resolver) resolveViaSSM(ctx context.Context, osImage *OSImage, arch string) (string, error) {
    ssmArch := arch
    if arch == "x86_64" {
        ssmArch = "amd64" // SSM uses amd64, EC2 uses x86_64
    }
    
    paramName := fmt.Sprintf(osImage.SSMPath, ssmArch)
    
    result, err := r.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
        Name: aws.String(paramName),
    })
    if err != nil {
        return "", fmt.Errorf("SSM lookup failed: %w", err)
    }
    
    return *result.Parameter.Value, nil
}

func (r *Resolver) resolveViaDescribeImages(ctx context.Context, osImage *OSImage, arch string) (string, error) {
    namePattern := fmt.Sprintf(osImage.NamePattern, arch)
    
    filters := []types.Filter{
        {
            Name:   aws.String("name"),
            Values: []string{namePattern},
        },
        {
            Name:   aws.String("architecture"),
            Values: []string{arch},
        },
        {
            Name:   aws.String("state"),
            Values: []string{"available"},
        },
    }
    
    var owners []string
    if osImage.OwnerID == "amazon" {
        owners = []string{"amazon"}
    } else {
        filters = append(filters, types.Filter{
            Name:   aws.String("owner-id"),
            Values: []string{osImage.OwnerID},
        })
    }
    
    input := &ec2.DescribeImagesInput{
        Filters: filters,
    }
    if len(owners) > 0 {
        input.Owners = owners
    }
    
    result, err := r.ec2Client.DescribeImages(ctx, input)
    if err != nil {
        return "", fmt.Errorf("DescribeImages failed: %w", err)
    }
    
    if len(result.Images) == 0 {
        return "", fmt.Errorf("no images found for %s in region %s", osImage.ID, r.region)
    }
    
    // Sort by creation date (newest first)
    sort.Slice(result.Images, func(i, j int) bool {
        return aws.ToString(result.Images[i].CreationDate) > aws.ToString(result.Images[j].CreationDate)
    })
    
    return aws.ToString(result.Images[0].ImageId), nil
}

func normalizeArch(arch string) string {
    switch strings.ToLower(arch) {
    case "amd64", "x86_64":
        return "x86_64"
    case "arm64", "aarch64":
        return "arm64"
    default:
        return arch
    }
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

### Phase 4: API Schema Updates

**File:** `api/holodeck/v1alpha1/types.go`

```go
// Instance defines an AWS instance
type Instance struct {
    Type   string `json:"type"`
    Region string `json:"region"`
    
    // OS specifies the operating system by ID (e.g., "ubuntu-22.04")
    // When set, AMI is automatically resolved for the region.
    // Takes precedence over Image if both are specified.
    // Run 'holodeck os list' for available options.
    // +optional
    OS string `json:"os,omitempty"`
    
    // Image allows explicit AMI specification
    // +optional
    Image Image `json:"image"`
    
    // ... other fields unchanged
}

// Auth defines authentication configuration
type Auth struct {
    KeyName string `json:"keyName"`
    
    // Username for SSH connection.
    // Auto-detected from OS if not specified.
    // +optional
    Username string `json:"username,omitempty"`
    
    PublicKey  string `json:"publicKey"`
    PrivateKey string `json:"privateKey"`
}
```

### Phase 5: Integration with AWS Provider

**File:** `pkg/provider/aws/aws.go`

```go
func (p *Provider) setAMI() error {
    // If explicit ImageId provided, use it (backward compatible)
    if p.Spec.Image.ImageId != nil {
        return nil
    }
    
    // If OS is specified, resolve via AMI resolver
    if p.Spec.OS != "" {
        arch := p.Spec.Image.Architecture
        if arch == "" {
            arch = "x86_64" // Default
        }
        
        resolved, err := p.amiResolver.Resolve(context.Background(), p.Spec.OS, arch)
        if err != nil {
            return fmt.Errorf("failed to resolve AMI for OS %s: %w", p.Spec.OS, err)
        }
        
        p.Spec.Image.ImageId = &resolved.ImageID
        
        // Auto-set username if not provided
        if p.Spec.Auth.Username == "" {
            p.Spec.Auth.Username = resolved.SSHUsername
        }
        
        return nil
    }
    
    // Fall back to existing logic (Ubuntu 22.04 default)
    // ... existing code ...
}
```

### Phase 6: CLI Commands

**New commands:**

```bash
# List available operating systems
holodeck os list

ID                   FAMILY    SSH USER    GPU SUPPORT
ubuntu-24.04         debian    ubuntu      ✓
ubuntu-22.04         debian    ubuntu      ✓
amazon-linux-2023    amazon    ec2-user    ✓
rocky-9              rhel      rocky       ✓

# Get details for a specific OS
holodeck os describe ubuntu-22.04

ID: ubuntu-22.04
Name: Ubuntu 22.04 LTS (Jammy Jellyfish)
Family: debian
SSH Username: ubuntu
Package Manager: apt
Min Root Volume: 20 GB
Architectures: x86_64, arm64

# Get AMI ID for specific region/arch
holodeck os ami ubuntu-22.04 --region us-west-2 --arch x86_64
ami-0abcdef1234567890
```

**Implementation:**

```go
// cmd/cli/os/list.go
func NewListCommand() *cli.Command {
    return &cli.Command{
        Name:  "list",
        Usage: "List available operating systems",
        Action: func(c *cli.Context) error {
            w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
            fmt.Fprintln(w, "ID\tFAMILY\tSSH USER\tARCHITECTURES")
            
            for _, img := range ami.All() {
                fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
                    img.ID,
                    img.Family,
                    img.SSHUsername,
                    strings.Join(img.Architectures, ", "),
                )
            }
            return w.Flush()
        },
    }
}
```

---

## Testing Strategy

### Unit Tests

1. **Registry tests** (`internal/ami/registry_test.go`)
   - Test `Get()` returns correct OS
   - Test `List()` returns all IDs
   - Test unknown OS returns error

2. **Resolver tests** (`internal/ami/resolver_test.go`)
   - Mock SSM client for SSM path tests
   - Mock EC2 client for DescribeImages tests
   - Test fallback from SSM to DescribeImages
   - Test architecture normalization
   - Test error cases

3. **Integration tests**
   - Test actual SSM lookups (requires AWS credentials)
   - Test DescribeImages lookups
   - Verify AMIs are accessible

### Test Cases

```go
var _ = Describe("AMI Resolver", func() {
    Context("Resolve", func() {
        It("should resolve ubuntu-22.04 via SSM", func() { ... })
        It("should fall back to DescribeImages when SSM fails", func() { ... })
        It("should return error for unknown OS", func() { ... })
        It("should normalize x86_64/amd64 architectures", func() { ... })
        It("should normalize arm64/aarch64 architectures", func() { ... })
        It("should return error for unsupported architecture", func() { ... })
    })
})
```

---

## Files to Create/Modify

### New Files
- `internal/ami/types.go` - Data types
- `internal/ami/registry.go` - OS registry
- `internal/ami/resolver.go` - AMI resolution logic
- `internal/ami/resolver_test.go` - Tests
- `cmd/cli/os/list.go` - `holodeck os list`
- `cmd/cli/os/describe.go` - `holodeck os describe`
- `cmd/cli/os/ami.go` - `holodeck os ami`

### Modified Files
- `api/holodeck/v1alpha1/types.go` - Add `OS` field to `Instance`
- `pkg/provider/aws/aws.go` - Add resolver, update `New()`
- `pkg/provider/aws/image.go` - Integrate resolver in `setAMI()`
- `cmd/cli/root.go` - Register `os` subcommand

---

## Acceptance Criteria

- [ ] Users can specify OS by simple ID (e.g., `ubuntu-22.04`)
- [ ] AMI is automatically resolved for region and architecture
- [ ] SSH username is automatically determined from OS
- [ ] `holodeck os list` shows available operating systems
- [ ] `holodeck os describe <id>` shows OS details
- [ ] `holodeck os ami <id> --region <r> --arch <a>` returns AMI ID
- [ ] Error messages help users find valid OS IDs
- [ ] Explicit `image.imageId` still works (backward compatible)
- [ ] At least 4 OS versions supported (ubuntu-22.04, ubuntu-24.04, amazon-linux-2023, rocky-9)
- [ ] Unit test coverage > 80% for new code

---

## Dependencies

- AWS SDK v2 for Go (`github.com/aws/aws-sdk-go-v2`)
  - `service/ec2` - already used
  - `service/ssm` - **new dependency** for SSM Parameter Store

---

## Future Enhancements (Out of Scope)

1. **Auto-update AMI registry** - CI job to refresh AMI IDs
2. **Custom OS definitions** - Allow users to define custom OS mappings
3. **AMI caching** - Cache resolved AMIs to reduce API calls
4. **OS version aliases** - `ubuntu-lts` → latest LTS version

---

## References

- [Issue #568](https://github.com/NVIDIA/holodeck/issues/568)
- [Canonical Ubuntu on AWS Documentation](https://documentation.ubuntu.com/aws/en/latest/aws-how-to/instances/find-ubuntu-images/)
- [AWS SSM Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html)
- [AWS EC2 DescribeImages API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html)
