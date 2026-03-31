# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.1] - 2026-03-31

### Bug Fixes

- **fix: HA NLB hairpin routing (#746, #762)** — Control-plane nodes now use `localhost:6443` for kubectl instead of the NLB endpoint, avoiding AWS NLB hairpin/loopback timeouts where a registered target connects through the NLB and gets routed back to itself.
- **fix: switch HA NLB to internal scheme (#760)** — NLB uses internal scheme to keep traffic within the VPC.
- **fix: NLB cleanup in periodic VPC cleaner (#762)** — `DeleteVPCResources` now deletes NLB listeners, target groups, and load balancers before attempting subnet/IGW/VPC deletion, preventing `DependencyViolation` errors from NLB-owned ENIs.

### CI

- **ci: update periodic cleanup to v0.3.0 and add manual trigger (#758)** — Periodic cleanup workflow uses the latest holodeck binary and supports manual dispatch.

## [v0.3.0] - 2026-03-30

### Features

#### Production-Grade Cluster Networking (#720–#728)

A complete overhaul of multi-node cluster networking for production workloads:

- Add cluster networking cache constants and struct fields (#720, @ArangoGutierrez)
- Add public subnet for cluster mode (#723, @ArangoGutierrez)
- Add Transport abstraction for SSH connections (#724, @ArangoGutierrez)
- Separate control-plane and worker security groups (#725, @ArangoGutierrez)
- Cleanup dual security groups on cluster delete (#726, @ArangoGutierrez)
- Wire SSM transport for private-subnet cluster nodes (#727, @ArangoGutierrez)
- Wire cluster networking — private subnets, NAT, hostForNode, real tests (#728, @ArangoGutierrez)
- Add ELBv2/NLB support for HA clusters (#614, @ArangoGutierrez)

#### Custom Templates (#701–#706)

User-defined provisioning templates with full lifecycle phase support:

- Add `CustomTemplate` type and `TemplatePhase` enum to API (#701, @ArangoGutierrez)
- Implement custom template loader and executor (#702, @ArangoGutierrez)
- Add custom template input validation (#703, @ArangoGutierrez)
- Integrate custom templates into dependency resolver (#706, @ArangoGutierrez)

#### RPM Support (#676–#681, #693)

First-class support for RPM-based Linux distributions including Rocky Linux 9, Amazon Linux 2023, and Fedora 42 across all runtime stacks (Docker, containerd, CRI-O):

- Add RPM support to Docker template (#677, @ArangoGutierrez)
- Add RPM support to NVIDIA driver template (#676, @ArangoGutierrez)
- Add RPM support to Kubernetes templates (#678, @ArangoGutierrez)
- Add RPM support to kernel template (#681, @ArangoGutierrez)
- Add RPM support to CRI-O template (#680, @ArangoGutierrez)
- Add RPM support to container-toolkit package template (#679, @ArangoGutierrez)
- Add RPM docs, e2e validation, and Fedora 42 support (#693, @ArangoGutierrez)
- Add DNF/YUM package manager support to provisioner

#### ARM64 Support

- Add AMI architecture detection and cross-validation (#664, @ArangoGutierrez)
- Infer AMI architecture from instance type for ARM64 support (#669, @ArangoGutierrez)
- Propagate image architecture in cluster mode (#661, @ArangoGutierrez)
- Add ARM64 GPU end-to-end test on merge to main (#670, @ArangoGutierrez)
- Detect Kubernetes arch at runtime instead of defaulting to amd64 (#663, @ArangoGutierrez)
- Use runtime arch for NVIDIA CUDA repository URL (#662, @ArangoGutierrez)
- Make architecture field case-insensitive for backward compatibility

#### Multi-Node Kubernetes Cluster Support (#562, #660)

- Add full multinode Kubernetes cluster support (#562, @ArangoGutierrez)
- Parallelize node provisioning, join info, and source/dest check (#660, @ArangoGutierrez)
- Fix cluster-mode OS resolution to use per-node specs

#### Multi-Source Installation (#635–#637)

- Add component provenance tracking to environment status (#635, @ArangoGutierrez)
- Support multiple installation sources for container runtimes (package, runfile, git) (#637, @ArangoGutierrez)
- Support multiple installation sources for NVIDIA drivers (package, runfile, git) (#636, @ArangoGutierrez)
- Add git ref resolution for CTK installation from GitHub sources
- Add Kubernetes installation from custom sources

#### CLI Improvements (#563, #621)

- Complete CLI with full CRUD operations (create, delete, list, status, dryrun) (#563, #621, @ArangoGutierrez)

#### AWS Infrastructure

- Add retry logic with exponential backoff (#616, @ArangoGutierrez)
- Add cleanup mode for standalone VPC cleanup to GitHub Action (#4938a2ee, @ArangoGutierrez)
- Replace bash cleanup script with native Go implementation (@ArangoGutierrez)
- Idempotent provisioning templates with enhanced error handling (#570, @ArangoGutierrez)
- Add Ubuntu 20.04 to OS AMI registry

#### Stale Issue Management (#695)

- Add GitHub workflow to mark issues as stale after 90 days of inactivity (#695, @ArangoGutierrez)

---

### Bug Fixes

#### Cluster Networking

- Wait for NAT Gateway available state before creating routes, fixing race condition (#735, @ArangoGutierrez)
- Verify API server against local IP before switching to NLB (#721, @ArangoGutierrez)
- Remove local kubeadm config cleanup that races in cluster mode (#718, @ArangoGutierrez)
- Make kubeadm config local path unique per environment (#717, @ArangoGutierrez)
- Guard substring slice and redact join credentials in logs (#654, @ArangoGutierrez)
- Surface NLB errors instead of swallowing them (#645, @ArangoGutierrez)
- Disable Source/Dest Check on ENI for single-node deployments
- Restrict security group CIDR to detected public IP (#615, @ArangoGutierrez)
- Fail instead of silently falling back to `0.0.0.0/0` for security group (#650, @ArangoGutierrez)
- Implement cleanup on partial creation failures (#612, @ArangoGutierrez)
- Propagate image architecture in cluster mode (#661)

#### Provisioner / SSH

- Close SSH client before reassign, close pipe reader (#659, @ArangoGutierrez)
- Split SSH sessions in `createKindConfig` (#657, @ArangoGutierrez)
- Remove unreachable code in `connectOrDie`, wrap last error (#655, @ArangoGutierrez)
- Correct retry count in `connectOrDie` error message (#628, @ArangoGutierrez)
- Guard nil provider for SSH provider mode (#643, @ArangoGutierrez)
- Close SSH client in `GetKubeConfig` (#640, @ArangoGutierrez)
- Use TOFU known_hosts for interactive SSH sessions (#653, @ArangoGutierrez)
- Add mutex to TOFU known_hosts to prevent race condition (#644, @ArangoGutierrez)
- Use TOFU host key verification in all SSH connections (#625, #630, @ArangoGutierrez)
- Log SFTP `MkdirAll` error instead of discarding (#642, @ArangoGutierrez)

#### Security

- Validate node labels and IPs before shell interpolation (#656, @ArangoGutierrez)
- Security and concurrency: template input validation, error wrapping (#623, @ArangoGutierrez)
- Critical bugs: `storageSizeGB` race, `log.Fatalf` in goroutine, file permissions (#622, @ArangoGutierrez)
- Create kubeconfig with 0600 permissions (#629, @ArangoGutierrez)
- Handle `crypto/rand` failure in retry jitter (#641, @ArangoGutierrez)
- Validate instance type before creating resources (#672, @ArangoGutierrez)

#### AWS Provider

- Preserve error chain with `errors.Join`, copy tags for goroutines (#651, @ArangoGutierrez)
- Return errors from `GenerateInstanceID`, validate ID format (#648, @ArangoGutierrez)
- Replace `context.TODO()` with proper timeouts in `create.go` (#611, @ArangoGutierrez)
- Add context propagation and timeout support to cleanup (#608, @ArangoGutierrez)
- Address ignored errors with logging and documentation (#610, @ArangoGutierrez)
- Use `%w` for proper error wrapping (#609, @ArangoGutierrez)
- Validate instance type before creating resources (#672)

#### Templates and Provisioning

- Pre-release validation fixes for kubeadm, networking, and CTK (#716, @ArangoGutierrez)
- E2E fixes for RPM-based distros (Rocky Linux 9)
- Validate feature gates, track branches, and endpoint host (#647, @ArangoGutierrez)
- Address audit fixes for heterogeneous cluster support
- Restore CTK/K8s validation and fix test schema

#### Logger / Utility

- Replace shared channels with per-invocation context cancellation (#631, @ArangoGutierrez)
- Nil out completed cancel entries in `activeCancels` (#646, @ArangoGutierrez)
- Replace signal goroutine with `signal.NotifyContext` (#649, @ArangoGutierrez)

#### Tests

- Use valid hex instance IDs in delete and status tests (#713, @ArangoGutierrez)

---

### Performance

- Inject sleep function to speed up AWS provider tests (~200x faster) (#739, @ArangoGutierrez)
- Parallelize node provisioning, join info, and source/dest check in cluster mode (#660, @ArangoGutierrez)
- Pre-compile templates, hoist regex, filter `DescribeLoadBalancers` (#652, @ArangoGutierrez)

---

### CI/CD

- Split E2E tests into smoke (pre-merge) and full (post-merge) tiers (#740, @ArangoGutierrez)
- Add ARM64 GPU end-to-end test on merge to main (#670, @ArangoGutierrez)
- Add workflow to mark issues as stale with no activity (#695, @ArangoGutierrez)
- Pin holodeck action to v0.2.18 in periodic CI workflow

---

### Documentation

- Update multinode cluster guide with production-grade networking (#734, @ArangoGutierrez)
- Comprehensive documentation refresh (#707, @ArangoGutierrez)
- Add multi-source installation guides and examples (#634, @ArangoGutierrez)
- Add OS selection guide
- Add CTK multi-source installation guide
- Add GitHub project management templates and AI instructions

---

### Dependencies

- Updated AWS SDK Go v2 to latest (multiple bumps: `aws-sdk-go-v2`, `service/ec2`, `service/ssm`, `service/elasticloadbalancingv2`, `config`)
- Updated Kubernetes libraries: `k8s.io/apimachinery`, `sigs.k8s.io/controller-runtime` to v0.23.3
- Updated `golang.org/x/crypto` to v0.49.0 and `golang.org/x/sync` to v0.20.0
- Updated Go base image from 1.25.5 to 1.26.1 (bookworm)
- Updated GitHub Actions: `docker/build-push-action` v7, `docker/setup-buildx-action` v4, `docker/metadata-action` v6, `docker/login-action` v4, `docker/setup-qemu-action` v4, `aws-actions/configure-aws-credentials` v6, `actions/upload-artifact` v7
- Updated `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` to latest
- Updated `github.com/pkg/sftp` to v1.13.10

---

## [v0.2.18] - 2025-12-13

See [GitHub Release](https://github.com/NVIDIA/holodeck/releases/tag/v0.2.18)
