# Contributing to Holodeck

Thank you for your interest in contributing to Holodeck! This guide will help
you get started.

## Development Setup

1. Fork the repository
1. Clone your fork:

   ```bash
   git clone https://github.com/your-username/holodeck.git
   cd holodeck
   ```

1. Add the upstream repository:

   ```bash
   git remote add upstream https://github.com/nvidia/holodeck.git
   ```

### Environment Requirements

- Linux or macOS (Windows is not supported)
- Go 1.20 or later
- Make
- Git

## Makefile Targets

The project uses a Makefile to manage common development tasks:

```bash
# Build the binary
make build

# Run tests
make test

# Run linters
make lint

# Clean build artifacts
make clean

# Run all checks (lint, test, build)
make check
```

## Running the CLI Locally

After building, you can run the CLI directly:

```bash
./bin/holodeck --help
```

Or install it system-wide:

```bash
sudo mv ./bin/holodeck /usr/local/bin/holodeck
```

## Development Workflow

1. Create a new branch for your feature/fix:

   ```bash
   git checkout -b feature/your-feature-name
   ```

1. Make your changes and commit them:

   ```bash
   git commit -s -m "feat: your feature description"
   ```

1. Push to your fork:

   ```bash
   git push origin feature/your-feature-name
   ```

1. Create a Pull Request against the main repository

## Commit Message Conventions

- Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
  - `feat: ...` for new features
  - `fix: ...` for bug fixes
  - `docs: ...` for documentation changes
  - `refactor: ...` for code refactoring
  - `test: ...` for adding or updating tests
  - `chore: ...` for maintenance
- Use the `-s` flag to sign off your commits

## Code Style

- Follow the Go code style guidelines
- Run `make lint` before submitting PRs
- Ensure all tests pass with `make test`

## Testing

- Write unit tests for new features
- Update existing tests when modifying features
- Run the full test suite with `make test`

## Documentation

- Update relevant documentation when adding features
- Follow the existing documentation style

## Pull Request Process

1. Ensure your PR description clearly describes the problem and solution
1. Include relevant issue numbers
1. Add tests for new functionality
1. Update documentation
1. Ensure CI passes

## Release Process

1. Version bump
1. Update changelog
1. Create release tag
1. Build and publish release artifacts

## Getting Help

- Open an issue for bugs or feature requests
- Join the community discussions
- Check existing documentation

## Code of Conduct

Please read and follow our [Code of Conduct](../CODE_OF_CONDUCT.md).
