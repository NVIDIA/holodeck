---
name: using-holodeck
description: Use when the user wants to provision, manage, or destroy GPU-enabled test environments via the holodeck CLI. Covers env.yaml config, create/list/status/ssh/scp/delete/cleanup workflows, and common pitfalls.
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

Required at minimum:

```yaml
apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: my-test-env
spec:
  instance:
    type: g4dn.xlarge        # GPU instance type
    region: us-west-2
    os: ubuntu-22.04         # or rhel-9, see 'holodeck os list'
  provider: aws              # or 'ssh' for pre-existing hosts
  kubernetes:
    install: true
    kubernetesVersion: v1.32.0
  nvidiaDriver:
    install: true
```

Run `holodeck os list` to discover supported OS identifiers.

## Core workflows

**Create + provision in one step:**
```bash
holodeck create -f env.yaml --provision
```

**Inspect what would happen first (dry-run):**
```bash
holodeck dryrun -f env.yaml
```

**List active environments:**
```bash
holodeck list
holodeck list -o json   # for scripts
```

**Get one environment's status:**
```bash
holodeck status <instance-id>
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

**Destroy:**
```bash
holodeck delete <instance-id>
```

**Clean up orphaned AWS VPC resources:**
```bash
holodeck cleanup vpc-12345678
```

## Output flags

Most read commands (`list`, `status`, `describe`) accept
`-o table|json|yaml` (default `table`). Scripts should use
`-o json`.

## Common pitfalls

- **Credentials** — AWS provider needs `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
  (or any other valid sdk credential source). SSH provider needs a
  reachable host and a private key.
- **Region** — instance `region` must match a region with available
  GPU capacity. `g4dn` and `p4` families have limited inventory
  in some regions; `us-west-2` and `us-east-1` are reliable.
- **OS** — only OS IDs listed by `holodeck os list` are valid.
  Custom AMIs need `spec.instance.amiID` set explicitly instead of `os`.
- **Cache** — instance metadata lives in `~/.cache/holodeck/` by default;
  `--cachepath` overrides. `list` shows only cached envs.
- **VPC leak** — failed provisions sometimes leave VPCs orphaned.
  Use `holodeck cleanup vpc-<id>` to remove them.

## Anti-patterns

- Don't run `create` without `--dry-run` against an unfamiliar
  config — provisioning failures cost real money.
- Don't share an env.yaml with embedded secrets — use env vars or
  AWS profiles instead.
- Don't manually `terraform destroy` against a holodeck-managed env;
  use `holodeck delete <id>`, which cleans up both infra and cache.
