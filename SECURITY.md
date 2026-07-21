# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.3.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it
responsibly.

**Please do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please report them via GitHub's private vulnerability reporting:

1. Go to the [Security Advisories page](https://github.com/NVIDIA/holodeck/security/advisories)
2. Click **"Report a vulnerability"**
3. Fill in the details

Alternatively, email **psirt@nvidia.com** with:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Any potential impact

## Response Timeline

- **Acknowledgment:** within 3 business days
- **Initial assessment:** within 10 business days
- **Fix timeline:** depends on severity, typically within 30-90 days

## Scope

Holodeck provisions **ephemeral, GPU-enabled cloud environments** (AWS EC2) for
end-to-end testing of NVIDIA Kubernetes components. Because it creates and tears
down real cloud infrastructure — VPCs, EC2 instances, security groups, and
IAM-scoped credentials — we take the security of that provisioning path, our
CI/CD pipelines, and our supply chain seriously.

Areas of particular interest:
- Cloud credential handling (AWS access keys, SSH key material, kubeconfig)
- Security group configuration (ingress to SSH and the Kubernetes API is
  restricted to the auto-detected caller IP `/32`, never `0.0.0.0/0`)
- IAM least-privilege for provisioned resources
- Supply chain integrity (dependencies, build process, release artifacts)
- GitHub Actions workflow security
- Container image vulnerabilities
