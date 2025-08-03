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
