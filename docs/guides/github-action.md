# GitHub Action Usage Guide

<!-- Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
Licensed under the Apache License, Version 2.0 -->

This guide covers how to use Holodeck as a GitHub Action to provision
GPU-ready cloud environments for CI workflows.

## Overview

The `NVIDIA/holodeck` GitHub Action builds and runs Holodeck inside a Docker
container. It reads a Holodeck configuration file from your repository and
provisions a full GPU test environment — including cloud infrastructure,
NVIDIA drivers, container runtime, and optionally Kubernetes — then tears it
down automatically when the workflow job finishes.

The action has two operating modes, controlled by the `action` input:

- **`create`** (default): provision an environment from a Holodeck config
  file, run any subsequent workflow steps against it, and clean up
  automatically via the post-entrypoint when the job completes.
- **`cleanup`**: perform a standalone VPC cleanup for one or more VPC IDs.
  Useful for periodic maintenance workflows that remove stale resources.

## Inputs Reference

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `action` | No | `create` | Action to perform: `create` or `cleanup`. |
| `holodeck_config` | Yes (create) | — | Path to the Holodeck `Environment` config file, relative to the repository root. |
| `aws_access_key_id` | No | — | AWS Access Key ID. Can be omitted if the runner already has AWS credentials configured. |
| `aws_secret_access_key` | No | — | AWS Secret Access Key. Can be omitted if the runner already has AWS credentials configured. |
| `aws_ssh_key` | No | — | PEM-encoded SSH private key used to connect to provisioned EC2 instances. |
| `vsphere_ssh_key` | No | — | SSH private key for vSphere-backed environments. |
| `vsphere_username` | No | — | vSphere/vCenter username. |
| `vsphere_password` | No | — | vSphere/vCenter password. |
| `vpc_ids` | No | — | Space-separated VPC IDs to clean up. Required when `action` is `cleanup`. |
| `aws_region` | No | — | AWS region for VPC cleanup operations. |
| `force_cleanup` | No | `false` | When `true`, skip GitHub job status checks and force-delete VPC resources. |

## Basic Usage: Provision, Test, Auto-Cleanup

The most common pattern provisions a GPU environment, runs your test suite
against it, and relies on the post-entrypoint to tear everything down when
the job finishes.

```yaml
name: GPU Integration Tests

on:
  push:
    branches: [main]
  pull_request:

jobs:
  gpu-test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Holodeck environment
        uses: NVIDIA/holodeck@main
        with:
          aws_access_key_id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws_secret_access_key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws_ssh_key: ${{ secrets.AWS_SSH_KEY }}
          holodeck_config: "tests/data/holodeck.yml"

      - name: Run GPU tests
        run: |
          # The kubeconfig is available at $GITHUB_WORKSPACE/kubeconfig
          # The provisioned host is accessible via the SSH key
          make test
```

### Holodeck Configuration File

The `holodeck_config` input points to an `Environment` manifest in your
repository. For example, `tests/data/holodeck.yml`:

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: gpu-ci
  description: "GPU integration test environment"
spec:
  provider: aws
  auth:
    keyName: my-key-pair
    privateKey: /home/runner/.cache/key
  instance:
    type: g4dn.xlarge
    region: us-east-1
    image:
      architecture: amd64
  containerRuntime:
    install: true
    name: docker
  nvidiaContainerToolkit:
    install: true
  nvidiaDriver:
    install: true
  kubernetes:
    install: true
    installer: kubeadm
```

The path in `holodeck_config` is relative to the repository root
(`$GITHUB_WORKSPACE`). The action prepends the workspace path automatically.

## Cleanup Mode: Periodic VPC Maintenance

Use `action: cleanup` in a scheduled workflow to remove stale VPCs that were
left behind by interrupted CI runs.

```yaml
name: Cleanup Stale VPCs

on:
  schedule:
    - cron: '0 3 * * *'   # daily at 03:00 UTC
  workflow_dispatch:
    inputs:
      force:
        description: 'Force cleanup without checking job status'
        required: false
        default: 'false'

jobs:
  cleanup:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Clean up stale VPCs
        uses: NVIDIA/holodeck@main
        with:
          action: cleanup
          aws_access_key_id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws_secret_access_key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws_region: us-east-1
          vpc_ids: ${{ vars.STALE_VPC_IDS }}   # space-separated list
          force_cleanup: ${{ github.event.inputs.force || 'false' }}
```

When `force_cleanup` is `false` (the default), Holodeck checks the GitHub
job status associated with each VPC before deleting it. Set `force_cleanup`
to `true` to skip that check and delete resources unconditionally.

## Tips

### Store credentials as repository secrets

Never hard-code AWS credentials or SSH keys in your workflow files. Store
them as [encrypted secrets](https://docs.github.com/en/actions/security-guides/using-secrets-in-github-actions)
and reference them with `${{ secrets.NAME }}`.

Required secrets for AWS environments:

| Secret | Description |
|--------|-------------|
| `AWS_ACCESS_KEY_ID` | IAM access key with EC2 permissions. |
| `AWS_SECRET_ACCESS_KEY` | Corresponding IAM secret key. |
| `AWS_SSH_KEY` | PEM-encoded private key matching the `keyName` in your config. |

### Self-hosted runners with pre-configured credentials

The `aws_access_key_id`, `aws_secret_access_key`, and `aws_ssh_key` inputs
are optional. If your self-hosted runner already has AWS credentials
configured (for example through an IAM instance role or environment
variables), you can omit those inputs entirely.

### Kubeconfig output location

When `kubernetes.install` is `true` in your config, the action writes the
kubeconfig to `$GITHUB_WORKSPACE/kubeconfig`. Subsequent steps can reference
it directly:

```yaml
- name: Run kubectl
  run: kubectl --kubeconfig=$GITHUB_WORKSPACE/kubeconfig get nodes
```

### Post-entrypoint auto-cleanup

The `action.yml` registers `holodeck` as both the `entrypoint` and the
`post-entrypoint`. When the job finishes — whether it passes, fails, or is
cancelled — GitHub Actions automatically invokes the post-entrypoint, which
calls the cleanup path and destroys the provisioned infrastructure. You do
not need a separate cleanup step in most workflows.

### Pinning to a specific version

Use a tagged release instead of `@main` for reproducible builds:

```yaml
uses: NVIDIA/holodeck@v0.3.0
```
