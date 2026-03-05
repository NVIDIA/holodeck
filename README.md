# Holodeck

[![Latest Release](https://img.shields.io/github/v/release/NVIDIA/holodeck?label=latest%20release)](https://github.com/NVIDIA/holodeck/releases/latest)

[![CI Pipeline](https://github.com/NVIDIA/holodeck/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/NVIDIA/holodeck/actions/workflows/ci.yaml)

A tool for creating and managing GPU-ready Cloud test environments.

---

## 📖 Documentation

- [Quick Start](docs/quick-start.md)
- [Prerequisites](docs/prerequisites.md)
- [Commands Reference](docs/commands/)
- [Contributing Guide](docs/contributing/)
- [Examples](docs/examples/)
- [Latest Release](https://github.com/NVIDIA/holodeck/releases/latest)

---

## ✨ Features

- **Multi-OS Support**: Ubuntu, Rocky Linux 9, Amazon Linux 2023 with
    automatic AMI resolution
    ([guide](docs/guides/os-selection.md))
- **Multi-Architecture**: x86_64 and ARM64 with automatic architecture
    inference
- **Custom Templates**: Run user-provided scripts at any provisioning
    phase ([guide](docs/guides/custom-templates.md))
- **Multi-Node Clusters**: HA Kubernetes clusters with kubeadm
    ([guide](docs/guides/multinode-clusters.md))
- **Flexible Sources**: Install components from packages, git, runfiles,
    or latest branches
    ([guide](docs/guides/source-selection.md))
- **Automatic IP Detection**: No manual IP configuration needed for AWS
    ([guide](docs/guides/ip-detection.md))

---

## 🚀 Quick Start

See [docs/quick-start.md](docs/quick-start.md) for a full walkthrough.

```bash
make build
sudo mv ./bin/holodeck /usr/local/bin/holodeck
holodeck --help
```

---

## 🛠️ Prerequisites

- Go 1.20+
- (For AWS) Valid AWS credentials in your environment
- (For SSH) Reachable host and valid SSH key

See [docs/prerequisites.md](docs/prerequisites.md) for details.

---

## ⚠️ Important: Kernel Compatibility

When installing NVIDIA drivers, Holodeck requires kernel headers matching your running kernel
version. If exact headers are unavailable, Holodeck will attempt to find compatible ones,
though this may cause driver compilation issues.

For kernel compatibility details and troubleshooting, see
[Kernel Compatibility](docs/prerequisites.md#kernel-compatibility) in the prerequisites documentation.

---

## 📝 How to Contribute

See [docs/contributing/](docs/contributing/) for full details.

### Main Makefile Targets

- `make build` – Build the holodeck binary
- `make test` – Run all tests
- `make lint` – Run linters
- `make clean` – Remove build artifacts

---

## 🧑‍💻 Usage

See [docs/commands/](docs/commands/) for detailed command documentation and examples.

```bash
holodeck --help
```

### Example: Create an environment

```bash
holodeck create -f ./examples/v1alpha1_environment.yaml
```

### Example: List environments

```bash
holodeck list
```

### Example: Delete an environment

```bash
holodeck delete <instance-id>
```

### Example: Clean up AWS VPC resources

```bash
holodeck cleanup vpc-12345678
```

### Example: Check status

```bash
holodeck status <instance-id>
```

### Example: Dry Run

```bash
holodeck dryrun -f ./examples/v1alpha1_environment.yaml
```

## 📂 More

- [Examples](docs/examples/)
- [Guides](docs/guides/)

---

For more information, see the [documentation](docs/README.md) directory.
