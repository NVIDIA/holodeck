# NVIDIA Container Toolkit Source Examples

This directory contains example configurations demonstrating the different NVIDIA Container Toolkit source options available in Holodeck.

## Source Types

### Package Source (`package`)
Installs from distro packages (default behavior).

- **`package-stable.yaml`**: Install from stable channel
- **`backward-compatible.yaml`**: Legacy configuration using version field

### Git Source (`git`)  
Installs from a specific git ref (commit, tag, branch, or PR).

- **`git-specific-tag.yaml`**: Install from a specific tag
- **`git-commit-sha.yaml`**: Install from a specific commit SHA with custom build configuration

### Latest Source (`latest`)
Installs from the latest commit of a specified branch.

- **`latest-main.yaml`**: Install from latest commit on main branch

## Features

- **Provenance Tracking**: Git and latest sources create `/etc/nvidia-container-toolkit/PROVENANCE.json` with build metadata
- **Custom Builds**: Git source supports custom make targets and environment variables
- **Default Repository**: When no repo is specified, defaults to `https://github.com/NVIDIA/nvidia-container-toolkit.git`
- **Backward Compatibility**: Existing `version` field continues to work as before

## Usage

```bash
holodeck create -f examples/nct-sources/package-stable.yaml
holodeck create -f examples/nct-sources/git-specific-tag.yaml
holodeck create -f examples/nct-sources/latest-main.yaml
```