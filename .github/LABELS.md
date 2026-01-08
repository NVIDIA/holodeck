# GitHub Labels Taxonomy

This document defines the hierarchical label system for the Holodeck project.
Labels follow a structured taxonomy to enable precise issue categorization,
filtering, and project management.

---

## Label Categories

### Priority (`prio/`)

Indicates urgency and scheduling priority.

| Label | Color | Description |
|-------|-------|-------------|
| `prio/p0-blocker` | `#b60205` (red) | Critical blocker - immediate attention required |
| `prio/p1-high` | `#d93f0b` (orange) | High priority - address soon |
| `prio/p2-medium` | `#fbca04` (yellow) | Medium priority - address when possible |
| `prio/p3-low` | `#0e8a16` (green) | Low priority - nice to have |

### Kind (`kind/`)

Classifies the type of issue or PR.

| Label | Color | Description |
|-------|-------|-------------|
| `kind/feature` | `#a2eeef` (cyan) | New feature or enhancement |
| `kind/bug` | `#d73a4a` (red) | Something isn't working |
| `kind/docs` | `#0075ca` (blue) | Documentation only changes |
| `kind/test` | `#bfd4f2` (light blue) | Test improvements or additions |
| `kind/refactor` | `#d4c5f9` (purple) | Code refactoring, no behavior change |
| `kind/chore` | `#fef2c0` (cream) | Maintenance, no code changes |
| `kind/security` | `#ee0701` (bright red) | Security-related issue |

### Area (`area/`)

Maps to project components and modules.

| Label | Color | Description |
|-------|-------|-------------|
| `area/provider-aws` | `#ff9900` (AWS orange) | AWS cloud provider functionality |
| `area/provider-ssh` | `#1d76db` (blue) | SSH provider functionality |
| `area/provisioner` | `#5319e7` (purple) | Software provisioning system |
| `area/cli` | `#006b75` (teal) | CLI commands and interface |
| `area/api` | `#0052cc` (dark blue) | Environment API and types |
| `area/action` | `#2188ff` (GitHub blue) | GitHub Action integration |
| `area/nvidia-driver` | `#76b900` (NVIDIA green) | NVIDIA driver installation |
| `area/container-runtime` | `#326ce5` (K8s blue) | Container runtime (docker, containerd, crio) |
| `area/kubernetes` | `#326ce5` (K8s blue) | Kubernetes installation and config |
| `area/container-toolkit` | `#76b900` (NVIDIA green) | NVIDIA Container Toolkit |
| `area/ci-cd` | `#ededed` (gray) | CI/CD pipelines and workflows |

### Size (`size/`)

Estimates effort for sprint planning. Based on lines of code changed.

| Label | Color | Description |
|-------|-------|-------------|
| `size/xs` | `#009800` (green) | Extra small: <10 lines |
| `size/s` | `#77dd77` (light green) | Small: 10-50 lines |
| `size/m` | `#fbca04` (yellow) | Medium: 50-200 lines |
| `size/l` | `#eb6420` (orange) | Large: 200-500 lines |
| `size/xl` | `#b60205` (red) | Extra large: >500 lines |

### Operations (`ops/`)

DevOps and operational concerns.

| Label | Color | Description |
|-------|-------|-------------|
| `ops/ci-cd` | `#ededed` (gray) | CI/CD pipelines, workflows, release automation |
| `ops/security` | `#ee0701` (bright red) | Security practices and hardening |
| `ops/infra` | `#c5def5` (light blue) | Infrastructure and deployment |

### Special Labels

| Label | Color | Description |
|-------|-------|-------------|
| `good first issue` | `#7057ff` (purple) | Good for newcomers |
| `help wanted` | `#008672` (green) | Extra attention is needed |
| `duplicate` | `#cfd3d7` (gray) | This issue already exists |
| `wontfix` | `#ffffff` (white) | This will not be worked on |
| `needs-triage` | `#d876e3` (pink) | Needs initial review and labeling |
| `blocked` | `#b60205` (red) | Blocked by external dependency |
| `stale` | `#fef2c0` (cream) | No recent activity |

---

## Usage Guidelines

### Issue Labeling

Every issue should have at minimum:
1. **One `kind/` label** - What type of work is this?
2. **One `prio/` label** - How urgent is this?
3. **One or more `area/` labels** - What components are affected?

### PR Labeling

Pull requests should have:
1. **One `kind/` label** - Matching the type of change
2. **One `size/` label** - Automatically applied by PR size workflow
3. **Relevant `area/` labels** - Components being modified

### Searching and Filtering

Examples of useful label queries:

```
# High priority bugs in AWS provider
label:"prio/p0-blocker","prio/p1-high" label:"kind/bug" label:"area/provider-aws"

# Good first issues for newcomers
label:"good first issue" -label:"blocked"

# All documentation work
label:"kind/docs"

# Small to medium refactoring tasks
label:"kind/refactor" label:"size/xs","size/s","size/m"
```

---

## Creating Labels

To create these labels in GitHub, you can use the GitHub CLI:

```bash
# Example: Create priority labels
gh label create "prio/p0-blocker" --color "b60205" --description "Critical blocker - immediate attention required"
gh label create "prio/p1-high" --color "d93f0b" --description "High priority - address soon"
gh label create "prio/p2-medium" --color "fbca04" --description "Medium priority - address when possible"
gh label create "prio/p3-low" --color "0e8a16" --description "Low priority - nice to have"
```

See the `scripts/create-labels.sh` script for automated label creation.

---

## Maintenance

- Labels should be reviewed quarterly for relevance
- New `area/` labels may be added as components are added
- Unused labels should be deprecated, not deleted (for historical tracking)
