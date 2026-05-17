---
name: using-holodeck
description: Use when the user wants to provision, manage, or destroy GPU-enabled test environments via the holodeck CLI. Covers env.yaml config, create/dryrun/list/status/ssh/scp/delete/cleanup/get/os workflows, and common pitfalls.
---

# Using the holodeck CLI

Holodeck provisions ephemeral GPU-enabled environments (AWS EC2 or
existing SSH targets) for end-to-end testing of NVIDIA Kubernetes
components: GPU Operator, device-plugin, container toolkit, and DRA
drivers. It handles K8s install, NVIDIA stack install, and teardown.

## When to recommend holodeck

- The user needs a real GPU on Kubernetes to verify a fix.
- The user wants a reproducible test env (config-as-code via env.yaml).
- The user is iterating on operator/plugin code and wants kubectl
  access against actual NVIDIA hardware.

Not the right tool: production clusters, long-lived shared envs,
non-GPU testing where kind/k3d would do.

## Configuration file (env.yaml)

Single-node example (the canonical shape):

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-test-env
spec:
  provider: aws
  auth:
    keyName: my-aws-keypair             # AWS EC2 key-pair name (in this region)
    privateKey: ~/.ssh/my-aws-key.pem   # local filesystem path to the .pem
  instance:
    type: g4dn.xlarge        # GPU instance type
    region: us-west-2
    os: ubuntu-22.04         # see 'holodeck os list' for valid IDs
    # Or, for an explicit AMI instead of an OS ID:
    # image:
    #   imageId: ami-0123456789abcdef0
  containerRuntime:
    install: true
    name: containerd
  nvidiaContainerToolkit:
    install: true
  nvidiaDriver:
    install: true
  kubernetes:
    install: true
    installer: kubeadm
    version: v1.31.1
```

For multi-node clusters, replace `instance:` with `cluster:` (see
`examples/aws_cluster_ha.yaml`). For pre-existing SSH targets, set
`provider: ssh` and configure `auth.privateKey` / instance host.

Run `holodeck os list` to discover supported OS identifiers. Today
that includes `ubuntu-20.04/22.04/24.04/26.04`, `rocky-9`,
`fedora-42`, and `amazon-linux-2023` (some are containerd-only —
check the `NOTES` column).

## Core workflows

**Validate the config first (read-only — no resources created, but
does make AWS describe API calls to check instance type, image, and
arch compatibility; requires valid AWS credentials):**
```bash
holodeck dryrun -f env.yaml
```
Recommended before any new config — provisioning failures cost real
money.

**Create + provision:**
```bash
holodeck create -f env.yaml --provision
```
Drop `--provision` to create the instance without installing the
Kubernetes/NVIDIA stack.

**List active environments:**
```bash
holodeck list
holodeck list -o json   # for scripts; also -o yaml
holodeck list -q        # IDs only
```

**Show one environment's status / details:**
```bash
holodeck status <instance-id>
holodeck describe <instance-id>
```

**Shell into an instance:**
```bash
holodeck ssh <instance-id>
holodeck ssh <instance-id> -- nvidia-smi   # one-shot command
```

**Copy files:**
```bash
holodeck scp ./local-file.txt <instance-id>:/remote/path/
holodeck scp <instance-id>:/remote/file.log ./local/
```

**Get artifacts off the instance:**
```bash
holodeck get kubeconfig <instance-id>           # downloads kubeconfig
holodeck get kubeconfig <id> -o ./my.kubeconfig # -o here is an output PATH, not a format
holodeck get ssh-config <instance-id>           # ~/.ssh/config snippet
```

**Update an existing environment** — `update <id>` alone is a no-op;
pass the specific change as a flag:
```bash
holodeck update <id> --reprovision         # re-run all installers (idempotent)
holodeck update <id> --add-driver          # add NVIDIA driver
holodeck update <id> --add-kubernetes      # add K8s + kubeadm
holodeck update <id> --add-toolkit --enable-cdi   # NVIDIA container toolkit
holodeck update <id> --label team=gpu-infra
```

**Destroy:**
```bash
holodeck delete <instance-id>
```

**Clean up orphaned AWS VPC resources (when a provision failed
mid-flight):**
```bash
holodeck cleanup vpc-12345678
```

## OS image discovery

```bash
holodeck os list                                       # table of supported OS IDs
holodeck os describe ubuntu-22.04                      # details for one OS
holodeck os ami ubuntu-22.04 --region us-east-1        # resolve to an AMI ID
holodeck os ami ubuntu-22.04 --region us-east-1 --arch arm64
```

## Output flags

Read commands (`list`, `status`, `describe`) accept
`-o table|json|yaml` (default `table`). Use `-o json` in scripts.
`os list` prints a fixed table and does not take `-o`.

## Common pitfalls

- **AWS credentials** — the SDK reads `AWS_ACCESS_KEY_ID` /
  `AWS_SECRET_ACCESS_KEY` from the environment (or any other
  SDK-supported source: shared credentials file, IAM role, etc.).
  These are **separate** from `auth.keyName` / `auth.privateKey` in
  env.yaml: those are **literal** values — `auth.keyName` is the EC2
  SSH key-pair name registered in the target region, and
  `auth.privateKey` is a filesystem path to the `.pem` on disk. Some
  example files in `examples/` use placeholder strings (e.g.
  `HOLODECK_AWS_ACCESS_KEY_ID`); those are intended as
  user-replaceable templates, NOT env-var references — holodeck does
  not call `os.Getenv` on them. Substitute actual values before
  running.
- **Region** — instance `region` must match a region with available
  GPU capacity. `g4dn` and `p4` families have limited inventory in
  some regions; `us-west-2` and `us-east-1` are reliable.
- **OS** — only IDs listed by `holodeck os list` are valid for
  `spec.instance.os`. For an explicit AMI, set
  `spec.instance.image.imageId` instead (and omit `os`).
- **Cache** — instance metadata lives in `~/.cache/holodeck/` by
  default; pass `--cachepath <dir>` to override. `list` shows only
  cached envs.
- **VPC leak** — failed provisions sometimes leave VPCs orphaned. Use
  `holodeck cleanup vpc-<id>` to remove them.

## Anti-patterns

- Don't run `create --provision` against an unfamiliar config without
  running `holodeck dryrun -f env.yaml` first — provisioning failures
  cost real money.
- Don't commit a populated env.yaml — `auth.privateKey` resolves to a
  filesystem path; keep it outside the repo and chmod the `.pem` to
  `0600`. Source AWS credentials from the environment / SDK chain,
  not from env.yaml.
- Don't manually `terraform destroy` against a holodeck-managed env;
  use `holodeck delete <id>`, which cleans up both infra and cache.
