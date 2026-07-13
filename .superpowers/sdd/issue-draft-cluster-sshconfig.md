Title: feat(provisioner): sshConfig support for cluster mode

## Background

#851 wired `auth.sshConfig` end-to-end for single-node provisioning (T8:
`provisioner.New` and the create/dryrun/CI ingestion paths now validate and
apply `knownHostsPolicy`, bastion, agent auth, and timeouts). Cluster mode
was explicitly left out of that wiring because the semantics are undesigned
for a multi-node topology.

T8b (this follow-up's predecessor) closed the resulting silent-ignore gap:
a cluster-mode `env.yaml` carrying `auth.sshConfig` is now rejected loudly
at every validation touchpoint —

- `EnvironmentSpec.ValidateSSHConfigMode()` (api/holodeck/v1alpha1/validation.go),
  the single source of truth for the rule, called from:
  - `cmd/cli/create/create.go` (Before hook, before any cloud action)
  - `cmd/cli/dryrun/dryrun.go` (Before hook, before any SSH action)
  - `cmd/action/ci/entrypoint.go` (before `provider.Create()`)
- `pkg/provisioner.NewClusterProvisioner` (sticky-error guard, surfaced by
  `ProvisionCluster` and `GetClusterHealth` before any node action) —
  defense in depth for any future caller that skips the ingestion layer.

Error returned: "auth.sshConfig is not yet supported in cluster mode (see
NVIDIA/holodeck#851); remove the sshConfig block or use single-node mode".

## The gap this issue tracks

`pkg/provisioner/cluster.go` has 7 in-package `New(...)` call sites (per-node
SSH provisioning during cluster bring-up: base provisioning, control-plane
init, control-plane join, worker join, and cluster-health checks). None of
them pass `WithSSHConfig`, so even once the rejection above is lifted for a
given cluster env, nothing currently threads `auth.sshConfig` through to
those per-node dials. This issue is to design and implement that wiring.

## Design questions to resolve before implementation

1. **Per-node vs. global bastion.** A single top-level `auth.sshConfig.bastion`
   assumes one jump host for the whole cluster. Real deployments may need
   different bastions per node pool (e.g. control-plane vs. worker subnets,
   or cross-AZ/cross-region clusters). Does cluster mode need a per-node or
   per-pool bastion override, or is a single cluster-wide bastion sufficient
   for v1?

2. **Interaction with `NodeInfo.Transport` / SSM precedence.** Cluster nodes
   already carry a `Transport` field (`pkg/provisioner/cluster.go`'s
   `NodeInfo`), used today for SSM-based access to private-subnet instances
   (see `transportOptsForNode`, `hostForNode`). `auth.sshConfig.bastion` and
   SSM are two different ways to reach a private node. When both are
   present, which wins? Should they be mutually exclusive per node (validate
   and reject the combination), or does SSM take precedence unconditionally
   since it doesn't need a bastion hop at all?

3. **Per-node host-key policy and known_hosts.** `knownHostsPolicy: strict`
   for single-node dials against one static host is well-defined. For a
   cluster with N ephemeral nodes (freshly created EC2 instances, new host
   keys every run), does `strict` even make sense, or should cluster mode
   force `accept-new` (TOFU) regardless of the configured policy? Needs a
   documented decision, not a silent override.

4. **Timeouts under fan-out.** `ProvisionCluster` dials multiple nodes
   (sequentially today, per `provisionBaseOnAllNodes`/`configureNodes`).
   `connectTimeout`/`handshakeTimeout` from `auth.sshConfig` were sized for
   a single dial in the single-node path (T8's N1 change). Does cluster mode
   need larger effective timeouts, or a total per-cluster budget, to avoid
   one slow node exhausting time for the rest?

5. **Agent auth across nodes.** `useAgent`/`agentSocket` target "the target
   hop" in the single-node model. In cluster mode there's no single target —
   should agent auth apply uniformly to every node, or is it out of scope
   given the bastion-fallback credential semantics already documented on
   `BastionConfig` (hop-1 reuses target Username/PrivateKey when unset)?

## Suggested next step

Answer the above (a short design note, not a full spec, should suffice)
before touching the 7 `New(...)` call sites in `pkg/provisioner/cluster.go`.
Once the shape is agreed, the wiring itself is mechanical: thread
`provisioner.WithSSHConfig(...)` (or a per-node derivative) through
`ProvisionCluster`'s node loop the same way T8 threaded it through the four
single-node `New(...)` sites, then lift the `ValidateSSHConfigMode` rejection
for the cases the design covers.

Ref: NVIDIA/holodeck#851
