/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package templates

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

// containerdV1Template is used for containerd 1.x versions (default 1.7.27)
// Supports both Debian-based (apt) and RHEL-based (dnf/yum) distributions
const containerdV1Template = `
COMPONENT="containerd"
SOURCE="package"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if containerd is already installed and functional
if systemctl is-active --quiet containerd 2>/dev/null; then
    INSTALLED_VERSION=$(containerd --version 2>/dev/null | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION."* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_containerd; then
                holodeck_log "INFO" "$COMPONENT" "Containerd verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Containerd installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Installing containerd {{.Version}} using package repository"

# Install required packages (OS-agnostic)
holodeck_retry 3 "$COMPONENT" pkg_update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry ca-certificates curl

# Source OS release info once for use in repository configuration
# shellcheck source=/etc/os-release
. /etc/os-release

# Add Docker/containerd repository based on OS family
case "${HOLODECK_OS_FAMILY}" in
    debian)
        # Debian/Ubuntu: Add Docker apt repository
        if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
            sudo install -m 0755 -d /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
                sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            sudo chmod a+r /etc/apt/keyrings/docker.gpg
        else
            holodeck_log "INFO" "$COMPONENT" "Docker GPG key already present"
        fi

        if [[ ! -f /etc/apt/sources.list.d/docker.list ]]; then
            echo \
              "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${ID} \
              ${VERSION_CODENAME} stable" | \
              sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
            holodeck_retry 3 "$COMPONENT" pkg_update
        else
            holodeck_log "INFO" "$COMPONENT" "Docker repository already configured"
        fi

        # Install containerd with specific version if provided
        if [[ -n "{{.Version}}" ]] && [[ "{{.Version}}" != "latest" ]]; then
            holodeck_log "INFO" "$COMPONENT" "Attempting to install containerd.io={{.Version}}-1"
            if ! holodeck_retry 3 "$COMPONENT" pkg_install_version "containerd.io" "{{.Version}}-1"; then
                holodeck_log "WARN" "$COMPONENT" \
                    "Specific version {{.Version}} not found, installing latest"
                holodeck_retry 3 "$COMPONENT" pkg_install containerd.io
            fi
        else
            holodeck_retry 3 "$COMPONENT" pkg_install containerd.io
        fi
        ;;

    amazon|rhel)
        # Amazon Linux / RHEL-based: Add Docker dnf/yum repository
        if [[ ! -f /etc/yum.repos.d/docker-ce.repo ]]; then
            case "${ID}" in
                amzn)
                    # Amazon Linux uses Fedora packages (Docker doesn't provide AL packages)
                    sudo curl -fsSL -o /etc/yum.repos.d/docker-ce.repo \
                        https://download.docker.com/linux/fedora/docker-ce.repo
                    # Replace $releasever with mapped Fedora version from common.go
                    sudo sed -i "s/\\\$releasever/${HOLODECK_AMZN_FEDORA_VERSION}/g" /etc/yum.repos.d/docker-ce.repo
                    holodeck_log "INFO" "$COMPONENT" "Using Fedora ${HOLODECK_AMZN_FEDORA_VERSION} repo for Amazon Linux"
                    ;;
                fedora)
                    sudo curl -fsSL -o /etc/yum.repos.d/docker-ce.repo \
                        https://download.docker.com/linux/fedora/docker-ce.repo
                    ;;
                *)
                    # Rocky, RHEL, CentOS, AlmaLinux
                    sudo curl -fsSL -o /etc/yum.repos.d/docker-ce.repo \
                        https://download.docker.com/linux/centos/docker-ce.repo
                    ;;
            esac
            holodeck_retry 3 "$COMPONENT" pkg_update
        else
            holodeck_log "INFO" "$COMPONENT" "Docker repository already configured"
        fi

        # Install containerd
        if [[ -n "{{.Version}}" ]] && [[ "{{.Version}}" != "latest" ]]; then
            holodeck_log "INFO" "$COMPONENT" "Attempting to install containerd.io-{{.Version}}"
            if ! holodeck_retry 3 "$COMPONENT" pkg_install_version "containerd.io" "{{.Version}}"; then
                holodeck_log "WARN" "$COMPONENT" \
                    "Specific version {{.Version}} not found, installing latest"
                holodeck_retry 3 "$COMPONENT" pkg_install containerd.io
            fi
        else
            holodeck_retry 3 "$COMPONENT" pkg_install containerd.io
        fi
        ;;

    *)
        holodeck_error 2 "$COMPONENT" \
            "Unsupported OS family: ${HOLODECK_OS_FAMILY}" \
            "Supported: debian, amazon, rhel"
        ;;
esac

holodeck_progress "$COMPONENT" 3 4 "Configuring containerd"

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Set systemd as the cgroup driver
sudo sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Restart containerd
sudo systemctl restart containerd
sudo systemctl enable containerd

holodeck_progress "$COMPONENT" 4 4 "Verifying installation"

# Wait for containerd to be ready with timeout (120s for slow VMs)
timeout=120
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for containerd to become ready" \
            "Check 'systemctl status containerd' and 'journalctl -u containerd'"
    fi
    if (( timeout % 15 == 0 )); then
        holodeck_log "INFO" "$COMPONENT" \
            "Waiting for containerd to become ready (${timeout}s remaining)"
    fi
    sleep 1
    ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" \
        "Containerd installation verification failed" \
        "Run 'systemctl status containerd' to diagnose"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd ${FINAL_VERSION}"
`

// containerdV2Template is used for containerd 2.x versions
// Based on official containerd installation guide for v2.x
const containerdV2Template = `
COMPONENT="containerd"
SOURCE="package"
DESIRED_VERSION="{{.Version}}"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

# Check if containerd is already installed and functional
if systemctl is-active --quiet containerd 2>/dev/null; then
    INSTALLED_VERSION=$(containerd --version 2>/dev/null | awk '{print $3}' || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ -z "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]] || \
           [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION."* ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            if holodeck_verify_containerd; then
                holodeck_log "INFO" "$COMPONENT" "Containerd verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Containerd installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Setting up prerequisites"

# Set up kernel modules (idempotent)
sudo modprobe overlay
sudo modprobe br_netfilter

# Setup required sysctl params (idempotent)
if [[ ! -f /etc/sysctl.d/99-kubernetes-cri.conf ]]; then
    cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF
    sudo sysctl --system
else
    holodeck_log "INFO" "$COMPONENT" "Sysctl params already configured"
fi

holodeck_progress "$COMPONENT" 3 6 "Installing dependencies"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry ca-certificates curl

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" ]]; then
    ARCH="arm64"
fi

holodeck_progress "$COMPONENT" 4 6 "Installing containerd {{.Version}} from official binaries"

# Download and install containerd (check if already installed)
if [[ ! -f /usr/local/bin/containerd ]] || \
   ! containerd --version 2>/dev/null | grep -q "{{.Version}}"; then
    CONTAINERD_TAR="containerd-{{.Version}}-linux-${ARCH}.tar.gz"
    CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v{{.Version}}/${CONTAINERD_TAR}"

    holodeck_log "INFO" "$COMPONENT" "Downloading containerd from $CONTAINERD_URL"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "${CONTAINERD_TAR}" "${CONTAINERD_URL}"
    sudo tar Cxzvf /usr/local "${CONTAINERD_TAR}"
    rm -f "${CONTAINERD_TAR}"
else
    holodeck_log "INFO" "$COMPONENT" "Containerd binary already at correct version"
fi

# Download and install runc (idempotent)
RUNC_VERSION="1.2.3"
if [[ ! -f /usr/local/sbin/runc ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing runc ${RUNC_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "runc.${ARCH}" \
        "https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"
    sudo install -m 755 "runc.${ARCH}" /usr/local/sbin/runc
    rm -f "runc.${ARCH}"
else
    holodeck_log "INFO" "$COMPONENT" "runc already installed"
fi

# Install CNI plugins (idempotent)
CNI_VERSION="v1.6.2"
if [[ ! -f /opt/cni/bin/bridge ]]; then
    CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
    holodeck_log "INFO" "$COMPONENT" "Installing CNI plugins ${CNI_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL -o "${CNI_TAR}" \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}"
    sudo mkdir -p /opt/cni/bin
    sudo tar Cxzvf /opt/cni/bin "${CNI_TAR}"
    rm -f "${CNI_TAR}"
else
    holodeck_log "INFO" "$COMPONENT" "CNI plugins already installed"
fi

holodeck_progress "$COMPONENT" 5 6 "Configuring containerd"

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null

# Update config for systemd cgroup
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Create containerd service (idempotent)
if [[ ! -f /etc/systemd/system/containerd.service ]]; then
    cat <<EOF | sudo tee /etc/systemd/system/containerd.service
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
fi

# Start containerd
sudo systemctl daemon-reload
sudo systemctl enable --now containerd

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

# Wait for containerd to be ready with timeout (120s for slow VMs)
timeout=120
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" \
            "Timeout waiting for containerd to become ready" \
            "Check 'systemctl status containerd' and 'journalctl -u containerd'"
    fi
    if (( timeout % 15 == 0 )); then
        holodeck_log "INFO" "$COMPONENT" \
            "Waiting for containerd to become ready (${timeout}s remaining)"
    fi
    sleep 1
    ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" \
        "Containerd installation verification failed" \
        "Run 'systemctl status containerd' to diagnose"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd ${FINAL_VERSION} (v2.x)"
`

// containerdGitTemplate builds and installs containerd from source.
const containerdGitTemplate = `
COMPONENT="containerd"
SOURCE="git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"

holodeck_progress "$COMPONENT" 1 6 "Checking existing installation"

# Check if already installed with this commit
if command -v containerd &>/dev/null; then
    if [[ -f /etc/containerd/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/containerd/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
                if holodeck_verify_containerd; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Installing build dependencies"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    build-essential ca-certificates curl git libseccomp-dev pkg-config

# Install Go toolchain
GO_VERSION="${CONTAINERD_GO_VERSION:-1.23.4}"
GO_ARCH="$(uname -m)"
case "${GO_ARCH}" in
    x86_64|amd64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    *) holodeck_log "ERROR" "$COMPONENT" "Unsupported arch: ${GO_ARCH}"; exit 1 ;;
esac
if ! command -v /usr/local/go/bin/go &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Go ${GO_VERSION}"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | \
        sudo tar -C /usr/local -xzf -
fi
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=auto

holodeck_progress "$COMPONENT" 3 6 "Cloning repository"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

if [[ -z "${GIT_REPO}" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "GIT_REPO is empty"
    exit 1
fi

if ! git clone --depth 1 "${GIT_REPO}" "${WORK_DIR}/src"; then
    holodeck_log "ERROR" "$COMPONENT" "Failed to clone ${GIT_REPO}"
    exit 1
fi
cd "${WORK_DIR}/src" || exit 1
if ! git fetch --depth 1 origin "${GIT_REF}"; then
    holodeck_log "ERROR" "$COMPONENT" "Failed to fetch ref ${GIT_REF}"
    exit 1
fi
git checkout FETCH_HEAD

holodeck_progress "$COMPONENT" 4 6 "Building from source"

if ! make; then
    holodeck_log "ERROR" "$COMPONENT" "Build failed"
    exit 1
fi

holodeck_progress "$COMPONENT" 5 6 "Installing binaries"

sudo make install

# Install runc if not present
RUNC_VERSION="1.2.3"
if [[ ! -f /usr/local/sbin/runc ]]; then
    ARCH="${GO_ARCH}"
    curl -fsSL -o "runc.${ARCH}" \
        "https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"
    sudo install -m 755 "runc.${ARCH}" /usr/local/sbin/runc
fi

# Install CNI plugins if not present
CNI_VERSION="v1.6.2"
if [[ ! -f /opt/cni/bin/bridge ]]; then
    ARCH="${GO_ARCH}"
    CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
    curl -fsSL -o "${CNI_TAR}" \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}"
    sudo mkdir -p /opt/cni/bin
    sudo tar Cxzvf /opt/cni/bin "${CNI_TAR}"
fi

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Create systemd service
if [[ ! -f /etc/systemd/system/containerd.service ]]; then
    cat <<EOF | sudo tee /etc/systemd/system/containerd.service
[Unit]
Description=containerd container runtime
After=network.target local-fs.target
[Service]
ExecStart=/usr/local/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
fi

sudo systemctl daemon-reload
sudo systemctl enable --now containerd

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

timeout=120
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" "Timeout waiting for containerd" \
            "Check 'systemctl status containerd'"
    fi
    sleep 1; ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" "Containerd verification failed after git build" \
        "Check build logs and 'systemctl status containerd'"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')

sudo mkdir -p /etc/containerd
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/containerd/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${FINAL_VERSION}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd ${FINAL_VERSION} from git: ${GIT_COMMIT}"
`

// containerdLatestTemplate tracks a branch at provision time.
const containerdLatestTemplate = `
COMPONENT="containerd"
SOURCE="latest"
GIT_REPO="{{.GitRepo}}"
TRACK_BRANCH="{{.TrackBranch}}"

holodeck_progress "$COMPONENT" 1 6 "Resolving latest commit on ${TRACK_BRANCH}"

if [[ -z "${GIT_REPO}" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "GIT_REPO is empty"
    exit 1
fi

if ! LATEST_COMMIT=$(git ls-remote "${GIT_REPO}" "refs/heads/${TRACK_BRANCH}" | cut -f1); then
    holodeck_log "ERROR" "$COMPONENT" "Failed to resolve ${TRACK_BRANCH} from ${GIT_REPO}"
    exit 1
fi
if [[ -z "$LATEST_COMMIT" ]]; then
    holodeck_log "ERROR" "$COMPONENT" "No commit found for branch ${TRACK_BRANCH}"
    exit 1
fi
SHORT_COMMIT="${LATEST_COMMIT:0:8}"
holodeck_log "INFO" "$COMPONENT" "Tracking ${TRACK_BRANCH} at ${SHORT_COMMIT}"

# Check if already at latest
if command -v containerd &>/dev/null; then
    if [[ -f /etc/containerd/PROVENANCE.json ]]; then
        if command -v jq &>/dev/null; then
            INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/containerd/PROVENANCE.json)
            if [[ "$INSTALLED_COMMIT" == "$SHORT_COMMIT" ]]; then
                if holodeck_verify_containerd; then
                    holodeck_log "INFO" "$COMPONENT" "Already at latest: ${SHORT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 6 "Installing build dependencies"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    build-essential ca-certificates curl git libseccomp-dev pkg-config

GO_VERSION="${CONTAINERD_GO_VERSION:-1.23.4}"
GO_ARCH="$(uname -m)"
case "${GO_ARCH}" in
    x86_64|amd64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    *) holodeck_log "ERROR" "$COMPONENT" "Unsupported arch: ${GO_ARCH}"; exit 1 ;;
esac
if ! command -v /usr/local/go/bin/go &>/dev/null; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | \
        sudo tar -C /usr/local -xzf -
fi
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=auto

holodeck_progress "$COMPONENT" 3 6 "Cloning repository at ${TRACK_BRANCH}"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

if ! git clone --depth 1 --branch "${TRACK_BRANCH}" "${GIT_REPO}" "${WORK_DIR}/src"; then
    holodeck_log "ERROR" "$COMPONENT" "Failed to clone ${GIT_REPO} branch ${TRACK_BRANCH}"
    exit 1
fi
cd "${WORK_DIR}/src" || exit 1

holodeck_progress "$COMPONENT" 4 6 "Building from source"

if ! make; then
    holodeck_log "ERROR" "$COMPONENT" "Build failed"
    exit 1
fi

holodeck_progress "$COMPONENT" 5 6 "Installing binaries"

sudo make install

# Install runc and CNI if not present (same as git template)
RUNC_VERSION="1.2.3"
if [[ ! -f /usr/local/sbin/runc ]]; then
    ARCH="${GO_ARCH}"
    curl -fsSL -o "runc.${ARCH}" \
        "https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"
    sudo install -m 755 "runc.${ARCH}" /usr/local/sbin/runc
fi
CNI_VERSION="v1.6.2"
if [[ ! -f /opt/cni/bin/bridge ]]; then
    ARCH="${GO_ARCH}"
    CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
    curl -fsSL -o "${CNI_TAR}" \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}"
    sudo mkdir -p /opt/cni/bin
    sudo tar Cxzvf /opt/cni/bin "${CNI_TAR}"
fi

# Configure and start containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

if [[ ! -f /etc/systemd/system/containerd.service ]]; then
    cat <<EOF | sudo tee /etc/systemd/system/containerd.service
[Unit]
Description=containerd container runtime
After=network.target local-fs.target
[Service]
ExecStart=/usr/local/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
fi

sudo systemctl daemon-reload
sudo systemctl enable --now containerd

holodeck_progress "$COMPONENT" 6 6 "Verifying installation"

timeout=120
while ! sudo ctr version &>/dev/null; do
    if [[ $timeout -le 0 ]]; then
        holodeck_error 11 "$COMPONENT" "Timeout waiting for containerd" \
            "Check 'systemctl status containerd'"
    fi
    sleep 1; ((timeout--))
done

if ! holodeck_verify_containerd; then
    holodeck_error 5 "$COMPONENT" "Containerd verification failed" \
        "Check build logs and 'systemctl status containerd'"
fi

FINAL_VERSION=$(containerd --version | awk '{print $3}')

sudo mkdir -p /etc/containerd
printf '%s\n' '{
  "source": "latest",
  "repo": "'"${GIT_REPO}"'",
  "branch": "'"${TRACK_BRANCH}"'",
  "commit": "'"${SHORT_COMMIT}"'",
  "version": "'"${FINAL_VERSION}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/containerd/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${FINAL_VERSION}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed containerd from ${TRACK_BRANCH}: ${SHORT_COMMIT}"
`

// Pre-compiled templates for containerd installation.
var (
	containerdV1Tmpl     = template.Must(template.New("containerd-v1").Parse(containerdV1Template))
	containerdV2Tmpl     = template.Must(template.New("containerd-v2").Parse(containerdV2Template))
	containerdGitTmpl    = template.Must(template.New("containerd-git").Parse(containerdGitTemplate))
	containerdLatestTmpl = template.Must(template.New("containerd-latest").Parse(containerdLatestTemplate))
)

// Containerd holds configuration for containerd installation.
type Containerd struct {
	// Source configuration
	Source string // "package", "git", "latest"

	// Package source fields
	Version      string
	MajorVersion int

	// Git source fields
	GitRepo   string
	GitRef    string
	GitCommit string // Resolved short SHA

	// Latest source fields
	TrackBranch string
}

// NewContainerd creates a Containerd from an Environment spec.
func NewContainerd(env v1alpha1.Environment) (*Containerd, error) {
	cr := env.Spec.ContainerRuntime

	c := &Containerd{
		Source: string(cr.Source),
	}

	// Default to package source
	if c.Source == "" {
		c.Source = "package"
	}

	switch c.Source {
	case "package":
		var version string
		switch {
		case cr.Package != nil && cr.Package.Version != "":
			version = cr.Package.Version
		case cr.Version != "":
			// Legacy field support
			version = cr.Version
		default:
			version = "1.7.27" // Default
		}
		version = strings.TrimPrefix(version, "v")

		// Parse major version
		c.MajorVersion = 1
		parts := strings.Split(version, ".")
		if len(parts) > 0 && parts[0] == "2" {
			c.MajorVersion = 2
			if version == "2" {
				version = "2.0.0"
			}
		}
		c.Version = version

	case "git":
		if cr.Git == nil {
			return nil, fmt.Errorf("git source requires 'git' configuration")
		}
		c.GitRepo = cr.Git.Repo
		c.GitRef = cr.Git.Ref
		if c.GitRepo == "" {
			c.GitRepo = "https://github.com/containerd/containerd.git"
		}

	case "latest":
		c.TrackBranch = "main"
		c.GitRepo = "https://github.com/containerd/containerd.git"
		if cr.Latest != nil {
			if cr.Latest.Track != "" {
				c.TrackBranch = cr.Latest.Track
			}
			if cr.Latest.Repo != "" {
				c.GitRepo = cr.Latest.Repo
			}
		}
	}

	return c, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (t *Containerd) SetResolvedCommit(shortSHA string) {
	t.GitCommit = shortSHA
}

// Execute renders the appropriate template based on source.
func (t *Containerd) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var tmpl *template.Template

	switch t.Source {
	case "package", "":
		if t.MajorVersion == 2 {
			tmpl = containerdV2Tmpl
		} else {
			tmpl = containerdV1Tmpl
		}
	case "git":
		tmpl = containerdGitTmpl
	case "latest":
		tmpl = containerdLatestTmpl
	default:
		return fmt.Errorf("unknown containerd source: %s", t.Source)
	}
	if err := tmpl.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute containerd template: %w", err)
	}
	return nil
}
