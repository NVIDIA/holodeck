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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const KubeadmTemplate = `
COMPONENT="kubernetes-kubeadm"
K8S_VERSION="{{.Version}}"
CNI_PLUGINS_VERSION="{{.CniPluginsVersion}}"
CALICO_VERSION="{{.CalicoVersion}}"
CRICTL_VERSION="{{.CrictlVersion}}"
ARCH="{{.Arch}}"
KUBELET_RELEASE_VERSION="{{.KubeletReleaseVersion}}"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 8 "Checking existing installation"

# Check if Kubernetes is already installed and functional
if [[ -f /etc/kubernetes/admin.conf ]]; then
    if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Kubernetes cluster already running"

        if holodeck_verify_kubernetes "/etc/kubernetes/admin.conf"; then
            # Verify all nodes are ready
            if kubectl --kubeconfig=/etc/kubernetes/admin.conf wait \
                --for=condition=ready --timeout=10s nodes --all &>/dev/null; then
                holodeck_log "INFO" "$COMPONENT" "Cluster verified functional"
                holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
                exit 0
            fi
        fi
        holodeck_log "WARN" "$COMPONENT" \
            "Cluster exists but not fully functional, attempting repair"
    fi
fi

holodeck_progress "$COMPONENT" 2 8 "Configuring system prerequisites"

# Disable swap (idempotent)
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Configure persistent loading of modules (idempotent)
if [[ ! -f /etc/modules-load.d/k8s.conf ]]; then
    sudo tee /etc/modules-load.d/k8s.conf > /dev/null <<EOF
overlay
br_netfilter
EOF
fi

# Load modules
sudo modprobe overlay
sudo modprobe br_netfilter

# Set up required sysctl params (idempotent)
if [[ ! -f /etc/sysctl.d/kubernetes.conf ]]; then
    sudo tee /etc/sysctl.d/kubernetes.conf > /dev/null <<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
    sudo sysctl --system
fi

holodeck_progress "$COMPONENT" 3 8 "Installing CNI plugins"

# Install CNI plugins (idempotent)
DEST="/opt/cni/bin"
sudo mkdir -p "$DEST"

if [[ ! -f "$DEST/bridge" ]] || [[ ! -f "$DEST/loopback" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing CNI plugins ${CNI_PLUGINS_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | \
        sudo tar -C "$DEST" -xz
    sudo chmod -R 755 "$DEST"
else
    holodeck_log "INFO" "$COMPONENT" "CNI plugins already installed"
fi

# Verify CNI plugins
if [[ ! -f "$DEST/bridge" ]] || [[ ! -f "$DEST/loopback" ]] || [[ ! -f "$DEST/portmap" ]]; then
    holodeck_error 4 "$COMPONENT" \
        "CNI plugins installation failed" \
        "Check network connectivity and try again"
fi

holodeck_progress "$COMPONENT" 4 8 "Installing Kubernetes binaries"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Install crictl (idempotent)
if [[ ! -f "$DOWNLOAD_DIR/crictl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing crictl ${CRICTL_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | \
        sudo tar -C "$DOWNLOAD_DIR" -xz
else
    holodeck_log "INFO" "$COMPONENT" "crictl already installed"
fi

# Install kubeadm, kubelet, kubectl (idempotent)
if [[ ! -f "$DOWNLOAD_DIR/kubeadm" ]] || [[ ! -f "$DOWNLOAD_DIR/kubelet" ]] || \
   [[ ! -f "$DOWNLOAD_DIR/kubectl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing kubeadm, kubelet, kubectl ${K8S_VERSION}"
    cd "$DOWNLOAD_DIR"
    sudo curl -L --remote-name-all \
        "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/{kubeadm,kubelet,kubectl}"
    sudo chmod +x kubeadm kubelet kubectl
else
    holodeck_log "INFO" "$COMPONENT" "Kubernetes binaries already installed"
fi

# Configure kubelet service (idempotent)
if [[ ! -f /etc/systemd/system/kubelet.service ]]; then
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service > /dev/null
fi

sudo mkdir -p /etc/systemd/system/kubelet.service.d
if [[ ! -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf ]]; then
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service.d/10-kubeadm.conf > /dev/null
fi

sudo systemctl daemon-reload
sudo systemctl enable --now kubelet

holodeck_progress "$COMPONENT" 5 8 "Initializing Kubernetes cluster"

# Initialize cluster only if not already initialized
if [[ ! -f /etc/kubernetes/admin.conf ]]; then
{{- if .UseLegacyInit }}
    # Using legacy kubeadm init for older Kubernetes versions
    holodeck_log "INFO" "$COMPONENT" "Using legacy kubeadm init"
    holodeck_retry 3 "$COMPONENT" sudo kubeadm init \
        --kubernetes-version="${K8S_VERSION}" \
        --pod-network-cidr=192.168.0.0/16 \
        --control-plane-endpoint="${K8S_ENDPOINT_HOST}:6443" \
        --ignore-preflight-errors=all
{{- else }}
    # Using kubeadm config file for newer Kubernetes versions
    holodeck_log "INFO" "$COMPONENT" "Using kubeadm config file"
    holodeck_retry 3 "$COMPONENT" sudo kubeadm init \
        --config /etc/kubernetes/kubeadm-config.yaml \
        --ignore-preflight-errors=all
{{- end }}
else
    holodeck_log "INFO" "$COMPONENT" "Cluster already initialized"
fi

# Setup kubeconfig
mkdir -p "$HOME/.kube"
sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="${HOME}/.kube/config"

holodeck_progress "$COMPONENT" 6 8 "Waiting for API server"

# Wait for kube-apiserver availability
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" version

holodeck_progress "$COMPONENT" 7 8 "Installing Calico CNI"

# Install Calico (idempotent)
if ! kubectl --kubeconfig "$KUBECONFIG" get namespace tigera-operator &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Calico ${CALICO_VERSION}"
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" create -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
else
    holodeck_log "INFO" "$COMPONENT" "Tigera operator already installed"
fi

# Wait for Tigera operator
holodeck_log "INFO" "$COMPONENT" "Waiting for Tigera operator"
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=available --timeout=300s deployment/tigera-operator -n tigera-operator

# Install Calico custom resources (idempotent)
if ! kubectl --kubeconfig "$KUBECONFIG" get installations.operator.tigera.io default \
    -n tigera-operator &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Calico custom resources"
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" apply -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml"
fi

# Wait for Calico
holodeck_log "INFO" "$COMPONENT" "Waiting for Calico"
holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-node -n calico-system
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-kube-controllers -n calico-system

holodeck_progress "$COMPONENT" 8 8 "Finalizing cluster configuration"

# Configure node for scheduling (idempotent - tolerates errors)
kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule- 2>/dev/null || true
kubectl label node --all node-role.kubernetes.io/worker= --overwrite 2>/dev/null || true
kubectl label node --all nvidia.com/holodeck.managed=true --overwrite 2>/dev/null || true

# Wait for cluster ready
holodeck_log "INFO" "$COMPONENT" "Waiting for cluster nodes"
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s nodes --all

# Wait for CoreDNS
holodeck_log "INFO" "$COMPONENT" "Waiting for CoreDNS"
holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=kube-dns -n kube-system

if ! holodeck_verify_kubernetes "$KUBECONFIG"; then
    holodeck_error 13 "$COMPONENT" \
        "Kubernetes cluster verification failed" \
        "Run 'kubectl get nodes' and 'kubectl get pods -A' to diagnose"
fi

holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed Kubernetes ${K8S_VERSION}"
`

const KindTemplate = `
COMPONENT="kubernetes-kind"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

# Check if KIND cluster already exists
if kind get clusters 2>/dev/null | grep -q "^holodeck$"; then
    holodeck_log "INFO" "$COMPONENT" "KIND cluster 'holodeck' already exists"

    # Verify cluster is functional
    export KUBECONFIG="${HOME}/.kube/config"
    if kubectl --kubeconfig "$KUBECONFIG" get nodes &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Cluster verified functional"
        holodeck_mark_installed "$COMPONENT" "kind"
        exit 0
    else
        holodeck_log "WARN" "$COMPONENT" \
            "Cluster exists but not functional, recreating"
        kind delete cluster --name holodeck || true
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Installing KIND"

# Download kind (idempotent)
if [[ ! -f /usr/local/bin/kind ]]; then
    ARCH_KIND=""
    if [[ "$(uname -m)" == "x86_64" ]]; then
        ARCH_KIND="amd64"
    elif [[ "$(uname -m)" == "aarch64" ]]; then
        ARCH_KIND="arm64"
    fi
    holodeck_retry 3 "$COMPONENT" curl -Lo ./kind \
        "https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-${ARCH_KIND}"
    chmod +x ./kind
    sudo install ./kind /usr/local/bin/kind
    rm -f ./kind
else
    holodeck_log "INFO" "$COMPONENT" "KIND already installed"
fi

holodeck_progress "$COMPONENT" 3 5 "Installing kubectl"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" ]]; then
    ARCH="arm64"
fi

# Install kubectl (idempotent)
if [[ ! -f "$DOWNLOAD_DIR/kubectl" ]]; then
    K8S_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
    holodeck_log "INFO" "$COMPONENT" "Installing kubectl ${K8S_VERSION}"
    cd "$DOWNLOAD_DIR"
    sudo curl -L --remote-name-all \
        "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/kubectl"
    sudo chmod +x kubectl
    cd "$HOME"
else
    holodeck_log "INFO" "$COMPONENT" "kubectl already installed"
fi

holodeck_progress "$COMPONENT" 4 5 "Configuring NVIDIA GPU support"

# Enable NVIDIA GPU support (idempotent)
if command -v nvidia-ctk &>/dev/null; then
    sudo nvidia-ctk runtime configure --set-as-default
    sudo systemctl restart docker
    sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts --in-place
else
    holodeck_log "WARN" "$COMPONENT" \
        "nvidia-ctk not found, skipping GPU configuration"
fi

holodeck_progress "$COMPONENT" 5 5 "Creating KIND cluster"

# Setup kubeconfig directory
mkdir -p "$HOME/.kube"
sudo chown -R "$(id -u):$(id -g)" "$HOME/.kube/"
export KUBECONFIG="${HOME}/.kube/config:/var/run/kubernetes/admin.kubeconfig"

# Prepare KIND config argument
KIND_CONFIG_ARGS=()
if [[ -n "{{.KindConfig}}" ]]; then
    KIND_CONFIG_ARGS=(--config /etc/kubernetes/kind.yaml)
fi

# Create cluster
holodeck_retry 3 "$COMPONENT" kind create cluster \
    --name holodeck \
    "${KIND_CONFIG_ARGS[@]}" \
    --kubeconfig="${HOME}/.kube/config"

# Verify cluster
if ! kubectl --kubeconfig "${HOME}/.kube/config" get nodes &>/dev/null; then
    holodeck_error 13 "$COMPONENT" \
        "KIND cluster creation verification failed" \
        "Run 'kind get clusters' and 'kubectl get nodes' to diagnose"
fi

holodeck_mark_installed "$COMPONENT" "kind"
holodeck_log "INFO" "$COMPONENT" "KIND cluster 'holodeck' installed successfully"
holodeck_log "INFO" "$COMPONENT" \
    "Access the cluster with: ssh -i <your-private-key> ubuntu@${K8S_ENDPOINT_HOST}"
`

// kindGitTemplate builds a custom node image from a git ref.
const kindGitTemplate = `
COMPONENT="kubernetes-kind-git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 7 "Checking existing installation"

# Check if already installed with this commit
if [[ -f /etc/kubernetes/PROVENANCE.json ]]; then
    if command -v jq &>/dev/null; then
        INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/kubernetes/PROVENANCE.json)
        if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
            if kind get clusters 2>/dev/null | grep -q "^holodeck$"; then
                export KUBECONFIG="${HOME}/.kube/config"
                if kubectl --kubeconfig "$KUBECONFIG" get nodes &>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

# Delete existing cluster if present
if kind get clusters 2>/dev/null | grep -q "^holodeck$"; then
    holodeck_log "INFO" "$COMPONENT" "Deleting existing cluster for rebuild"
    kind delete cluster --name holodeck || true
fi

holodeck_progress "$COMPONENT" 2 7 "Installing KIND and kubectl"

ARCH_KIND=""
if [[ "$(uname -m)" == "x86_64" ]]; then
    ARCH_KIND="amd64"
elif [[ "$(uname -m)" == "aarch64" ]]; then
    ARCH_KIND="arm64"
fi

# Install KIND
if [[ ! -f /usr/local/bin/kind ]]; then
    holodeck_retry 3 "$COMPONENT" curl -Lo ./kind \
        "https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-${ARCH_KIND}"
    chmod +x ./kind
    sudo install ./kind /usr/local/bin/kind
    rm -f ./kind
fi

# Install kubectl (required for cluster verification)
if [[ ! -f /usr/local/bin/kubectl ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing kubectl"
    KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
    holodeck_retry 3 "$COMPONENT" curl -Lo ./kubectl \
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH_KIND}/kubectl"
    chmod +x ./kubectl
    sudo install ./kubectl /usr/local/bin/kubectl
    rm -f ./kubectl
fi

# Ensure docker buildx is available for KIND node image builds
if ! docker buildx version &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing docker buildx"
    BUILDX_ARCH=""
    if [[ "$(uname -m)" == "x86_64" ]]; then
        BUILDX_ARCH="amd64"
    elif [[ "$(uname -m)" == "aarch64" ]]; then
        BUILDX_ARCH="arm64"
    fi
    mkdir -p ~/.docker/cli-plugins
    holodeck_retry 3 "$COMPONENT" curl -Lo ~/.docker/cli-plugins/docker-buildx \
        "https://github.com/docker/buildx/releases/download/v0.12.1/buildx-v0.12.1.linux-${BUILDX_ARCH}"
    chmod +x ~/.docker/cli-plugins/docker-buildx
fi

holodeck_progress "$COMPONENT" 3 7 "Cloning Kubernetes repository"

WORK_DIR=$(mktemp -d)

# Cleanup function that handles Go module cache read-only files
cleanup_workdir() {
    if [[ -d "$WORK_DIR" ]]; then
        chmod -R u+w "$WORK_DIR" 2>/dev/null || true
        rm -rf "$WORK_DIR" 2>/dev/null || true
    fi
}
trap cleanup_workdir EXIT

# Validate repository URL
if [[ -z "${GIT_REPO}" ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO is empty" ""
fi
if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}" ""
fi

holodeck_log "INFO" "$COMPONENT" "Cloning ${GIT_REPO} at ${GIT_REF}"
if ! git clone --depth 1 "${GIT_REPO}" "${WORK_DIR}/kubernetes" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to clone repository" "Check network and repo URL"
fi
cd "${WORK_DIR}/kubernetes" || exit 1
# Fetch tags for KIND version detection
git fetch --depth 1 --tags origin 2>&1 || true
if ! git fetch --depth 1 origin "${GIT_REF}" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to fetch ref ${GIT_REF}" "Verify ref exists"
fi
git checkout FETCH_HEAD

K8S_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "${GIT_COMMIT}")
holodeck_log "INFO" "$COMPONENT" "Detected K8s version: ${K8S_VERSION}"

holodeck_progress "$COMPONENT" 4 7 "Building KIND node image (this may take 20-40 minutes)"

NODE_IMAGE="holodeck/k8s-node:${GIT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Building node image: ${NODE_IMAGE}"

if ! kind build node-image "${WORK_DIR}/kubernetes" --image "${NODE_IMAGE}"; then
    holodeck_error 4 "$COMPONENT" "Failed to build KIND node image" "Check build logs above"
fi

holodeck_progress "$COMPONENT" 5 7 "Configuring NVIDIA GPU support"

if command -v nvidia-ctk &>/dev/null; then
    sudo nvidia-ctk runtime configure --set-as-default
    sudo systemctl restart docker
    sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts --in-place
else
    holodeck_log "WARN" "$COMPONENT" "nvidia-ctk not found, skipping GPU configuration"
fi

holodeck_progress "$COMPONENT" 6 7 "Creating KIND cluster"

mkdir -p "$HOME/.kube"
sudo chown -R "$(id -u):$(id -g)" "$HOME/.kube/"
export KUBECONFIG="${HOME}/.kube/config"

KIND_CONFIG_ARGS=()
if [[ -n "{{.KindConfig}}" ]]; then
    KIND_CONFIG_ARGS=(--config /etc/kubernetes/kind.yaml)
fi

holodeck_retry 3 "$COMPONENT" kind create cluster \
    --name holodeck \
    --image "${NODE_IMAGE}" \
    "${KIND_CONFIG_ARGS[@]}" \
    --kubeconfig="${HOME}/.kube/config"

holodeck_progress "$COMPONENT" 7 7 "Verifying cluster"

if ! kubectl --kubeconfig "${HOME}/.kube/config" get nodes &>/dev/null; then
    holodeck_error 13 "$COMPONENT" "KIND cluster verification failed" ""
fi

# Write provenance
sudo mkdir -p /etc/kubernetes
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${K8S_VERSION}"'",
  "installer": "kind",
  "node_image": "'"${NODE_IMAGE}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/kubernetes/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${GIT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "KIND cluster with custom K8s ${GIT_COMMIT} installed"
holodeck_log "INFO" "$COMPONENT" \
    "Access the cluster with: ssh -i <your-private-key> ubuntu@${K8S_ENDPOINT_HOST}"
`

// kindLatestTemplate tracks a branch and builds node image at provision time.
const kindLatestTemplate = `
COMPONENT="kubernetes-kind-latest"
GIT_REPO="{{.GitRepo}}"
TRACK_BRANCH="{{.TrackBranch}}"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 7 "Resolving latest commit on ${TRACK_BRANCH}"

# Validate repository URL
if [[ -z "${GIT_REPO}" ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO is empty" ""
fi
if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}" ""
fi

LATEST_COMMIT=$(git ls-remote "${GIT_REPO}" "refs/heads/${TRACK_BRANCH}" | cut -f1)
if [[ -z "$LATEST_COMMIT" ]]; then
    holodeck_error 2 "$COMPONENT" "Failed to resolve branch ${TRACK_BRANCH}" ""
fi
SHORT_COMMIT="${LATEST_COMMIT:0:8}"
holodeck_log "INFO" "$COMPONENT" "Tracking ${TRACK_BRANCH} at ${SHORT_COMMIT}"

# Check if already at latest
if [[ -f /etc/kubernetes/PROVENANCE.json ]]; then
    if command -v jq &>/dev/null; then
        INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/kubernetes/PROVENANCE.json)
        if [[ "$INSTALLED_COMMIT" == "$SHORT_COMMIT" ]]; then
            if kind get clusters 2>/dev/null | grep -q "^holodeck$"; then
                export KUBECONFIG="${HOME}/.kube/config"
                if kubectl --kubeconfig "$KUBECONFIG" get nodes &>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" "Already at latest: ${SHORT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

# Delete existing cluster for rebuild
if kind get clusters 2>/dev/null | grep -q "^holodeck$"; then
    holodeck_log "INFO" "$COMPONENT" "Deleting existing cluster for rebuild"
    kind delete cluster --name holodeck || true
fi

holodeck_progress "$COMPONENT" 2 7 "Installing KIND and kubectl"

ARCH_KIND=""
if [[ "$(uname -m)" == "x86_64" ]]; then
    ARCH_KIND="amd64"
elif [[ "$(uname -m)" == "aarch64" ]]; then
    ARCH_KIND="arm64"
fi

# Install KIND
if [[ ! -f /usr/local/bin/kind ]]; then
    holodeck_retry 3 "$COMPONENT" curl -Lo ./kind \
        "https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-${ARCH_KIND}"
    chmod +x ./kind
    sudo install ./kind /usr/local/bin/kind
    rm -f ./kind
fi

# Install kubectl (required for cluster verification)
if [[ ! -f /usr/local/bin/kubectl ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing kubectl"
    KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
    holodeck_retry 3 "$COMPONENT" curl -Lo ./kubectl \
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH_KIND}/kubectl"
    chmod +x ./kubectl
    sudo install ./kubectl /usr/local/bin/kubectl
    rm -f ./kubectl
fi

# Ensure docker buildx is available for KIND node image builds
if ! docker buildx version &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing docker buildx"
    BUILDX_ARCH=""
    if [[ "$(uname -m)" == "x86_64" ]]; then
        BUILDX_ARCH="amd64"
    elif [[ "$(uname -m)" == "aarch64" ]]; then
        BUILDX_ARCH="arm64"
    fi
    mkdir -p ~/.docker/cli-plugins
    holodeck_retry 3 "$COMPONENT" curl -Lo ~/.docker/cli-plugins/docker-buildx \
        "https://github.com/docker/buildx/releases/download/v0.12.1/buildx-v0.12.1.linux-${BUILDX_ARCH}"
    chmod +x ~/.docker/cli-plugins/docker-buildx
fi

holodeck_progress "$COMPONENT" 3 7 "Cloning Kubernetes repository"

WORK_DIR=$(mktemp -d)

# Cleanup function that handles Go module cache read-only files
cleanup_workdir() {
    if [[ -d "$WORK_DIR" ]]; then
        chmod -R u+w "$WORK_DIR" 2>/dev/null || true
        rm -rf "$WORK_DIR" 2>/dev/null || true
    fi
}
trap cleanup_workdir EXIT

holodeck_log "INFO" "$COMPONENT" "Cloning ${GIT_REPO} branch ${TRACK_BRANCH}"
if ! git clone --depth 1 --branch "${TRACK_BRANCH}" "${GIT_REPO}" "${WORK_DIR}/kubernetes" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to clone repository" "Check network and branch name"
fi
cd "${WORK_DIR}/kubernetes" || exit 1
# Fetch tags for KIND version detection
git fetch --depth 1 --tags origin 2>&1 || true

K8S_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "${SHORT_COMMIT}")
holodeck_log "INFO" "$COMPONENT" "Detected K8s version: ${K8S_VERSION}"

holodeck_progress "$COMPONENT" 4 7 "Building KIND node image (this may take 20-40 minutes)"

NODE_IMAGE="holodeck/k8s-node:${SHORT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Building node image: ${NODE_IMAGE}"

if ! kind build node-image "${WORK_DIR}/kubernetes" --image "${NODE_IMAGE}"; then
    holodeck_error 4 "$COMPONENT" "Failed to build KIND node image" ""
fi

holodeck_progress "$COMPONENT" 5 7 "Configuring NVIDIA GPU support"

if command -v nvidia-ctk &>/dev/null; then
    sudo nvidia-ctk runtime configure --set-as-default
    sudo systemctl restart docker
    sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts --in-place
else
    holodeck_log "WARN" "$COMPONENT" "nvidia-ctk not found, skipping GPU configuration"
fi

holodeck_progress "$COMPONENT" 6 7 "Creating KIND cluster"

mkdir -p "$HOME/.kube"
sudo chown -R "$(id -u):$(id -g)" "$HOME/.kube/"
export KUBECONFIG="${HOME}/.kube/config"

KIND_CONFIG_ARGS=()
if [[ -n "{{.KindConfig}}" ]]; then
    KIND_CONFIG_ARGS=(--config /etc/kubernetes/kind.yaml)
fi

holodeck_retry 3 "$COMPONENT" kind create cluster \
    --name holodeck \
    --image "${NODE_IMAGE}" \
    "${KIND_CONFIG_ARGS[@]}" \
    --kubeconfig="${HOME}/.kube/config"

holodeck_progress "$COMPONENT" 7 7 "Verifying cluster"

if ! kubectl --kubeconfig "${HOME}/.kube/config" get nodes &>/dev/null; then
    holodeck_error 13 "$COMPONENT" "KIND cluster verification failed" ""
fi

sudo mkdir -p /etc/kubernetes
printf '%s\n' '{
  "source": "latest",
  "repo": "'"${GIT_REPO}"'",
  "branch": "'"${TRACK_BRANCH}"'",
  "commit": "'"${SHORT_COMMIT}"'",
  "version": "'"${K8S_VERSION}"'",
  "installer": "kind",
  "node_image": "'"${NODE_IMAGE}"'",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/kubernetes/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${SHORT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "KIND cluster with ${TRACK_BRANCH}@${SHORT_COMMIT} installed"
holodeck_log "INFO" "$COMPONENT" \
    "Access the cluster with: ssh -i <your-private-key> ubuntu@${K8S_ENDPOINT_HOST}"
`

const microk8sTemplate = `
COMPONENT="kubernetes-microk8s"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"
K8S_VERSION="{{.Version}}"

# Remove leading 'v' from version if present for microk8s snap channel
MICROK8S_VERSION="${K8S_VERSION#v}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if MicroK8s is already installed
if snap list microk8s &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "MicroK8s already installed"

    # Verify cluster is functional
    if sudo microk8s status --wait-ready &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "MicroK8s cluster verified functional"
        holodeck_mark_installed "$COMPONENT" "$MICROK8S_VERSION"
        exit 0
    else
        holodeck_log "WARN" "$COMPONENT" \
            "MicroK8s installed but not functional, attempting repair"
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Installing MicroK8s"

holodeck_retry 3 "$COMPONENT" sudo apt-get update
holodeck_retry 3 "$COMPONENT" sudo snap install microk8s \
    --classic --channel="${MICROK8S_VERSION}"

holodeck_progress "$COMPONENT" 3 4 "Enabling MicroK8s addons"

# Enable addons
holodeck_retry 3 "$COMPONENT" sudo microk8s enable gpu dashboard dns registry

holodeck_progress "$COMPONENT" 4 4 "Configuring access"

# Configure user access
sudo usermod -a -G microk8s ubuntu || true
mkdir -p ~/.kube
sudo chown -f -R ubuntu ~/.kube
sudo microk8s config > ~/.kube/config
sudo chown -f -R ubuntu ~/.kube
sudo snap alias microk8s.kubectl kubectl || true

# Wait for cluster to be ready
holodeck_log "INFO" "$COMPONENT" "Waiting for MicroK8s to be ready"
sudo microk8s status --wait-ready

# Verify installation
if ! sudo microk8s kubectl get nodes &>/dev/null; then
    holodeck_error 13 "$COMPONENT" \
        "MicroK8s installation verification failed" \
        "Run 'sudo microk8s status' and 'sudo microk8s kubectl get nodes' to diagnose"
fi

holodeck_mark_installed "$COMPONENT" "$MICROK8S_VERSION"
holodeck_log "INFO" "$COMPONENT" "MicroK8s ${MICROK8S_VERSION} installed successfully"
holodeck_log "INFO" "$COMPONENT" \
    "Access the cluster with: ssh -i <your-private-key> ubuntu@${K8S_ENDPOINT_HOST}"
`

const kubeadmTemplate = `apiVersion: kubeadm.k8s.io/v1beta4
kind: InitConfiguration
nodeRegistration:
  criSocket: "{{ .CriSocket }}"
---
apiVersion: kubeadm.k8s.io/v1beta4
kind: ClusterConfiguration
clusterName: "{{ .ClusterName }}"
kubernetesVersion: "{{.KubernetesVersion}}"
controlPlaneEndpoint: "{{.ControlPlaneEndpoint}}:6443"
networking:
  podSubnet: "{{ .PodSubnet }}"
{{- if .FeatureGates }}
apiServer:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
    - name: "runtime-config"
      value: "{{ .RuntimeConfig }}"
controllerManager:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
scheduler:
  extraArgs:
    - name: "feature-gates"
      value: "{{ .FeatureGates }}"
{{- end }}
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
{{- if .IsUbuntu }}
resolvConf: /run/systemd/resolve/resolv.conf
{{- end }}
{{- if .ParsedFeatureGates }}
featureGates:
  {{- range $key, $value := .ParsedFeatureGates }}
  {{ $key }}: {{ $value }}
  {{- end }}
{{- end }}
`

// kubeadmGitTemplate builds Kubernetes from source and installs via kubeadm.
const kubeadmGitTemplate = `
COMPONENT="kubernetes-kubeadm-git"
GIT_REPO="{{.GitRepo}}"
GIT_REF="{{.GitRef}}"
GIT_COMMIT="{{.GitCommit}}"
CNI_PLUGINS_VERSION="{{.CniPluginsVersion}}"
CALICO_VERSION="{{.CalicoVersion}}"
CRICTL_VERSION="{{.CrictlVersion}}"
ARCH="{{.Arch}}"
KUBELET_RELEASE_VERSION="{{.KubeletReleaseVersion}}"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 11 "Checking existing installation"

# Check if already installed with this commit
if [[ -f /etc/kubernetes/PROVENANCE.json ]]; then
    if command -v jq &>/dev/null; then
        INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/kubernetes/PROVENANCE.json)
        if [[ "$INSTALLED_COMMIT" == "$GIT_COMMIT" ]]; then
            if [[ -f /etc/kubernetes/admin.conf ]]; then
                if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes &>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" "Already installed from commit: ${GIT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 11 "Checking system resources"

# Check available disk space (need at least 10GB for build)
AVAILABLE_GB=$(df -BG /tmp | tail -1 | awk '{print $4}' | sed 's/G//')
if [[ "$AVAILABLE_GB" -lt 10 ]]; then
    holodeck_error 1 "$COMPONENT" \
        "Insufficient disk space: ${AVAILABLE_GB}GB available, need 10GB+" \
        "Free up disk space or use a larger instance"
fi
holodeck_log "INFO" "$COMPONENT" "Available disk space: ${AVAILABLE_GB}GB"

# Check available memory
AVAILABLE_MEM_MB=$(free -m | awk '/^Mem:/{print $7}')
holodeck_log "INFO" "$COMPONENT" "Available memory: ${AVAILABLE_MEM_MB}MB"

holodeck_progress "$COMPONENT" 3 11 "Configuring system prerequisites"

# Disable swap (idempotent)
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Configure persistent loading of modules (idempotent)
if [[ ! -f /etc/modules-load.d/k8s.conf ]]; then
    sudo tee /etc/modules-load.d/k8s.conf > /dev/null <<EOF
overlay
br_netfilter
EOF
fi

sudo modprobe overlay
sudo modprobe br_netfilter

if [[ ! -f /etc/sysctl.d/kubernetes.conf ]]; then
    sudo tee /etc/sysctl.d/kubernetes.conf > /dev/null <<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
    sudo sysctl --system
fi

holodeck_progress "$COMPONENT" 4 11 "Installing build prerequisites"

# Install ALL required build tools
install_packages_with_retry make rsync gcc g++ libc6-dev jq

# Verify critical tools are available
for tool in make rsync gcc; do
    if ! command -v "$tool" &>/dev/null; then
        holodeck_error 2 "$COMPONENT" "Required tool not found: $tool" "Install build-essential"
    fi
done

holodeck_progress "$COMPONENT" 5 11 "Installing CNI plugins"

DEST="/opt/cni/bin"
sudo mkdir -p "$DEST"

if [[ ! -f "$DEST/bridge" ]] || [[ ! -f "$DEST/loopback" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing CNI plugins ${CNI_PLUGINS_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | \
        sudo tar -C "$DEST" -xz
    sudo chmod -R 755 "$DEST"
fi

holodeck_progress "$COMPONENT" 6 11 "Cloning Kubernetes repository"

WORK_DIR=$(mktemp -d)

# Cleanup function that handles Go module cache read-only files
cleanup_workdir() {
    if [[ -d "$WORK_DIR" ]]; then
        # Go module cache has read-only files, make writable before removing
        chmod -R u+w "$WORK_DIR" 2>/dev/null || true
        rm -rf "$WORK_DIR" 2>/dev/null || true
    fi
}
trap cleanup_workdir EXIT

# Validate repository URL (security: only allow GitHub URLs)
if [[ -z "${GIT_REPO}" ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO is empty" "Check environment configuration"
fi
if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}" ""
fi

holodeck_log "INFO" "$COMPONENT" "Cloning ${GIT_REPO} at ${GIT_REF}"
if ! git clone --depth 1 "${GIT_REPO}" "${WORK_DIR}/kubernetes" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to clone repository" "Check network and repo URL"
fi
cd "${WORK_DIR}/kubernetes" || exit 1
if ! git fetch --depth 1 origin "${GIT_REF}" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to fetch ref ${GIT_REF}" "Verify ref exists in repo"
fi
git checkout FETCH_HEAD

holodeck_progress "$COMPONENT" 7 11 "Installing Go toolchain"

# Extract Go version from go.mod
GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')
if [[ -z "$GO_VERSION" ]]; then
    GO_VERSION="1.23.4"  # Fallback
fi
holodeck_log "INFO" "$COMPONENT" "Required Go version: ${GO_VERSION}"

# Detect architecture
GO_ARCH="$(uname -m)"
case "${GO_ARCH}" in
    x86_64|amd64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    *)
        holodeck_error 3 "$COMPONENT" "Unsupported architecture: ${GO_ARCH}" ""
        ;;
esac

# Install Go if needed or version mismatch
NEED_GO_INSTALL=false
if ! command -v /usr/local/go/bin/go &>/dev/null; then
    NEED_GO_INSTALL=true
else
    INSTALLED_GO=$(/usr/local/go/bin/go version | awk '{print $3}' | sed 's/go//')
    if [[ "$INSTALLED_GO" != "$GO_VERSION"* ]]; then
        holodeck_log "INFO" "$COMPONENT" "Go version mismatch: installed=$INSTALLED_GO, required=$GO_VERSION"
        sudo rm -rf /usr/local/go
        NEED_GO_INSTALL=true
    fi
fi

if [[ "$NEED_GO_INSTALL" == "true" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing Go ${GO_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL \
        "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
fi

# Set Go environment - use local toolchain only (no auto-download)
export GOROOT="/usr/local/go"
export GOPATH="${WORK_DIR}/gopath"
export GOCACHE="${WORK_DIR}/gocache"
export GOTOOLCHAIN="local"
export PATH="${GOROOT}/bin:${GOPATH}/bin:$PATH"

# Verify Go is working
if ! go version; then
    holodeck_error 3 "$COMPONENT" "Go installation failed" "Check Go installation"
fi
holodeck_log "INFO" "$COMPONENT" "Using Go: $(go version)"

holodeck_progress "$COMPONENT" 8 11 "Building Kubernetes binaries (10-20 minutes)"

BUILD_LOG="${WORK_DIR}/build.log"
holodeck_log "INFO" "$COMPONENT" "Building kubeadm, kubelet, kubectl..."
holodeck_log "INFO" "$COMPONENT" "Build log: ${BUILD_LOG}"

# Run build with output capture
if ! make WHAT="cmd/kubeadm cmd/kubelet cmd/kubectl" 2>&1 | tee "$BUILD_LOG"; then
    holodeck_log "ERROR" "$COMPONENT" "Build failed. Last 100 lines of build log:"
    tail -100 "$BUILD_LOG" >&2
    holodeck_error 4 "$COMPONENT" "Kubernetes build failed" "Check build prerequisites and disk space"
fi

# Verify binaries were created
for binary in kubeadm kubelet kubectl; do
    if [[ ! -f "_output/bin/${binary}" ]]; then
        holodeck_error 4 "$COMPONENT" "Build incomplete: ${binary} not found" "Check build log"
    fi
done

# Get version string
K8S_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "${GIT_COMMIT}")
holodeck_log "INFO" "$COMPONENT" "Built Kubernetes version: ${K8S_VERSION}"

holodeck_progress "$COMPONENT" 9 11 "Installing Kubernetes binaries"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Install built binaries
sudo install -m 755 _output/bin/kubeadm "$DOWNLOAD_DIR/"
sudo install -m 755 _output/bin/kubelet "$DOWNLOAD_DIR/"
sudo install -m 755 _output/bin/kubectl "$DOWNLOAD_DIR/"

# Verify installation
for binary in kubeadm kubelet kubectl; do
    if ! "${DOWNLOAD_DIR}/${binary}" version --client 2>/dev/null || \
       ! "${DOWNLOAD_DIR}/${binary}" --version 2>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Installed ${binary}"
    fi
done

# Install crictl
if [[ ! -f "$DOWNLOAD_DIR/crictl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing crictl ${CRICTL_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | \
        sudo tar -C "$DOWNLOAD_DIR" -xz
fi

# Configure kubelet service
if [[ ! -f /etc/systemd/system/kubelet.service ]]; then
    holodeck_log "INFO" "$COMPONENT" "Configuring kubelet service"
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service > /dev/null
fi

sudo mkdir -p /etc/systemd/system/kubelet.service.d
if [[ ! -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf ]]; then
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service.d/10-kubeadm.conf > /dev/null
fi

sudo systemctl daemon-reload
sudo systemctl enable kubelet

holodeck_progress "$COMPONENT" 10 11 "Initializing Kubernetes cluster"

if [[ ! -f /etc/kubernetes/admin.conf ]]; then
    holodeck_log "INFO" "$COMPONENT" "Running kubeadm init"
    
    # Start kubelet before kubeadm init
    sudo systemctl start kubelet || true
    
    if ! sudo kubeadm init \
        --pod-network-cidr=192.168.0.0/16 \
        --control-plane-endpoint="${K8S_ENDPOINT_HOST}:6443" \
        --ignore-preflight-errors=all 2>&1 | tee /tmp/kubeadm-init.log; then
        holodeck_log "ERROR" "$COMPONENT" "kubeadm init failed. Output:"
        cat /tmp/kubeadm-init.log >&2
        holodeck_error 5 "$COMPONENT" "kubeadm init failed" "Check kubelet logs: journalctl -xeu kubelet"
    fi
fi

mkdir -p "$HOME/.kube"
sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="${HOME}/.kube/config"

holodeck_log "INFO" "$COMPONENT" "Waiting for API server..."
# Use get --raw /healthz instead of version to avoid client version parsing issues
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" get --raw /healthz

holodeck_progress "$COMPONENT" 11 11 "Installing CNI and finalizing"

# Install Calico CNI
if ! kubectl --kubeconfig "$KUBECONFIG" get namespace tigera-operator &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Calico ${CALICO_VERSION}"
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" create -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
fi

holodeck_log "INFO" "$COMPONENT" "Waiting for Tigera operator..."
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=available --timeout=300s deployment/tigera-operator -n tigera-operator

if ! kubectl --kubeconfig "$KUBECONFIG" get installations.operator.tigera.io default \
    -n tigera-operator &>/dev/null; then
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" apply -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml"
fi

holodeck_log "INFO" "$COMPONENT" "Waiting for Calico pods..."
holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-node -n calico-system
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-kube-controllers -n calico-system

# Configure node
kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule- 2>/dev/null || true
kubectl label node --all node-role.kubernetes.io/worker= --overwrite 2>/dev/null || true
kubectl label node --all nvidia.com/holodeck.managed=true --overwrite 2>/dev/null || true

holodeck_log "INFO" "$COMPONENT" "Waiting for nodes to be ready..."
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s nodes --all

holodeck_log "INFO" "$COMPONENT" "Waiting for CoreDNS..."
holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=kube-dns -n kube-system

# Write provenance
sudo mkdir -p /etc/kubernetes
printf '%s\n' '{
  "source": "git",
  "repo": "'"${GIT_REPO}"'",
  "ref": "'"${GIT_REF}"'",
  "commit": "'"${GIT_COMMIT}"'",
  "version": "'"${K8S_VERSION}"'",
  "installer": "kubeadm",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/kubernetes/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${GIT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed Kubernetes from git: ${GIT_COMMIT}"
`

// kubeadmLatestTemplate tracks a branch and builds at provision time.
const kubeadmLatestTemplate = `
COMPONENT="kubernetes-kubeadm-latest"
GIT_REPO="{{.GitRepo}}"
TRACK_BRANCH="{{.TrackBranch}}"
CNI_PLUGINS_VERSION="{{.CniPluginsVersion}}"
CALICO_VERSION="{{.CalicoVersion}}"
CRICTL_VERSION="{{.CrictlVersion}}"
ARCH="{{.Arch}}"
KUBELET_RELEASE_VERSION="{{.KubeletReleaseVersion}}"
K8S_ENDPOINT_HOST="{{.K8sEndpointHost}}"

holodeck_progress "$COMPONENT" 1 11 "Resolving latest commit on ${TRACK_BRANCH}"

# Validate repository URL
if [[ -z "${GIT_REPO}" ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO is empty" ""
fi
if [[ "${GIT_REPO}" != https://github.com/* && "${GIT_REPO}" != git@github.com:* ]]; then
    holodeck_error 1 "$COMPONENT" "GIT_REPO must be a GitHub URL: ${GIT_REPO}" ""
fi

# Resolve branch to latest commit
LATEST_COMMIT=$(git ls-remote "${GIT_REPO}" "refs/heads/${TRACK_BRANCH}" | cut -f1)
if [[ -z "$LATEST_COMMIT" ]]; then
    holodeck_error 2 "$COMPONENT" \
        "Failed to resolve branch ${TRACK_BRANCH}" "Verify branch exists in ${GIT_REPO}"
fi
SHORT_COMMIT="${LATEST_COMMIT:0:8}"
holodeck_log "INFO" "$COMPONENT" "Tracking ${TRACK_BRANCH} at ${SHORT_COMMIT}"

# Check if already at latest
if [[ -f /etc/kubernetes/PROVENANCE.json ]]; then
    if command -v jq &>/dev/null; then
        INSTALLED_COMMIT=$(jq -r '.commit // empty' /etc/kubernetes/PROVENANCE.json)
        if [[ "$INSTALLED_COMMIT" == "$SHORT_COMMIT" ]]; then
            if [[ -f /etc/kubernetes/admin.conf ]]; then
                if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes &>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" "Already at latest: ${SHORT_COMMIT}"
                    exit 0
                fi
            fi
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 11 "Checking system resources"

# Check available disk space
AVAILABLE_GB=$(df -BG /tmp | tail -1 | awk '{print $4}' | sed 's/G//')
if [[ "$AVAILABLE_GB" -lt 10 ]]; then
    holodeck_error 1 "$COMPONENT" \
        "Insufficient disk space: ${AVAILABLE_GB}GB available, need 10GB+" ""
fi
holodeck_log "INFO" "$COMPONENT" "Available disk space: ${AVAILABLE_GB}GB"

holodeck_progress "$COMPONENT" 3 11 "Configuring system prerequisites"

sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

if [[ ! -f /etc/modules-load.d/k8s.conf ]]; then
    sudo tee /etc/modules-load.d/k8s.conf > /dev/null <<EOF
overlay
br_netfilter
EOF
fi

sudo modprobe overlay
sudo modprobe br_netfilter

if [[ ! -f /etc/sysctl.d/kubernetes.conf ]]; then
    sudo tee /etc/sysctl.d/kubernetes.conf > /dev/null <<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
    sudo sysctl --system
fi

holodeck_progress "$COMPONENT" 4 11 "Installing build prerequisites"

install_packages_with_retry make rsync gcc g++ libc6-dev jq

holodeck_progress "$COMPONENT" 5 11 "Installing CNI plugins"

DEST="/opt/cni/bin"
sudo mkdir -p "$DEST"

if [[ ! -f "$DEST/bridge" ]] || [[ ! -f "$DEST/loopback" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing CNI plugins ${CNI_PLUGINS_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | \
        sudo tar -C "$DEST" -xz
    sudo chmod -R 755 "$DEST"
fi

holodeck_progress "$COMPONENT" 6 11 "Cloning Kubernetes repository"

WORK_DIR=$(mktemp -d)

# Cleanup function that handles Go module cache read-only files
cleanup_workdir() {
    if [[ -d "$WORK_DIR" ]]; then
        chmod -R u+w "$WORK_DIR" 2>/dev/null || true
        rm -rf "$WORK_DIR" 2>/dev/null || true
    fi
}
trap cleanup_workdir EXIT

holodeck_log "INFO" "$COMPONENT" "Cloning ${GIT_REPO} branch ${TRACK_BRANCH}"
if ! git clone --depth 1 --branch "${TRACK_BRANCH}" "${GIT_REPO}" "${WORK_DIR}/kubernetes" 2>&1; then
    holodeck_error 2 "$COMPONENT" "Failed to clone repository" "Check network and branch name"
fi
cd "${WORK_DIR}/kubernetes" || exit 1

holodeck_progress "$COMPONENT" 7 11 "Installing Go toolchain"

GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')
if [[ -z "$GO_VERSION" ]]; then
    GO_VERSION="1.23.4"
fi
holodeck_log "INFO" "$COMPONENT" "Required Go version: ${GO_VERSION}"

GO_ARCH="$(uname -m)"
case "${GO_ARCH}" in
    x86_64|amd64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    *)
        holodeck_error 3 "$COMPONENT" "Unsupported architecture: ${GO_ARCH}" ""
        ;;
esac

NEED_GO_INSTALL=false
if ! command -v /usr/local/go/bin/go &>/dev/null; then
    NEED_GO_INSTALL=true
else
    INSTALLED_GO=$(/usr/local/go/bin/go version | awk '{print $3}' | sed 's/go//')
    if [[ "$INSTALLED_GO" != "$GO_VERSION"* ]]; then
        sudo rm -rf /usr/local/go
        NEED_GO_INSTALL=true
    fi
fi

if [[ "$NEED_GO_INSTALL" == "true" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing Go ${GO_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -fsSL \
        "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
fi

export GOROOT="/usr/local/go"
export GOPATH="${WORK_DIR}/gopath"
export GOCACHE="${WORK_DIR}/gocache"
export GOTOOLCHAIN="local"
export PATH="${GOROOT}/bin:${GOPATH}/bin:$PATH"

holodeck_log "INFO" "$COMPONENT" "Using Go: $(go version)"

holodeck_progress "$COMPONENT" 8 11 "Building Kubernetes binaries (10-20 minutes)"

BUILD_LOG="${WORK_DIR}/build.log"
holodeck_log "INFO" "$COMPONENT" "Building kubeadm, kubelet, kubectl..."

if ! make WHAT="cmd/kubeadm cmd/kubelet cmd/kubectl" 2>&1 | tee "$BUILD_LOG"; then
    holodeck_log "ERROR" "$COMPONENT" "Build failed. Last 100 lines:"
    tail -100 "$BUILD_LOG" >&2
    holodeck_error 4 "$COMPONENT" "Kubernetes build failed" "Check build log"
fi

for binary in kubeadm kubelet kubectl; do
    if [[ ! -f "_output/bin/${binary}" ]]; then
        holodeck_error 4 "$COMPONENT" "Build incomplete: ${binary} not found" ""
    fi
done

K8S_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "${SHORT_COMMIT}")
holodeck_log "INFO" "$COMPONENT" "Built Kubernetes version: ${K8S_VERSION}"

holodeck_progress "$COMPONENT" 9 11 "Installing Kubernetes binaries"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

sudo install -m 755 _output/bin/kubeadm "$DOWNLOAD_DIR/"
sudo install -m 755 _output/bin/kubelet "$DOWNLOAD_DIR/"
sudo install -m 755 _output/bin/kubectl "$DOWNLOAD_DIR/"

if [[ ! -f "$DOWNLOAD_DIR/crictl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing crictl ${CRICTL_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | \
        sudo tar -C "$DOWNLOAD_DIR" -xz
fi

if [[ ! -f /etc/systemd/system/kubelet.service ]]; then
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service > /dev/null
fi

sudo mkdir -p /etc/systemd/system/kubelet.service.d
if [[ ! -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf ]]; then
    curl -sSL \
        "https://raw.githubusercontent.com/kubernetes/release/${KUBELET_RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" | \
        sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | \
        sudo tee /etc/systemd/system/kubelet.service.d/10-kubeadm.conf > /dev/null
fi

sudo systemctl daemon-reload
sudo systemctl enable kubelet

holodeck_progress "$COMPONENT" 10 11 "Initializing Kubernetes cluster"

if [[ ! -f /etc/kubernetes/admin.conf ]]; then
    sudo systemctl start kubelet || true
    
    if ! sudo kubeadm init \
        --pod-network-cidr=192.168.0.0/16 \
        --control-plane-endpoint="${K8S_ENDPOINT_HOST}:6443" \
        --ignore-preflight-errors=all 2>&1 | tee /tmp/kubeadm-init.log; then
        holodeck_log "ERROR" "$COMPONENT" "kubeadm init failed:"
        cat /tmp/kubeadm-init.log >&2
        holodeck_error 5 "$COMPONENT" "kubeadm init failed" "Check kubelet: journalctl -xeu kubelet"
    fi
fi

mkdir -p "$HOME/.kube"
sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="${HOME}/.kube/config"

# Use get --raw /healthz instead of version to avoid client version parsing issues
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" get --raw /healthz

holodeck_progress "$COMPONENT" 11 11 "Installing CNI and finalizing"

if ! kubectl --kubeconfig "$KUBECONFIG" get namespace tigera-operator &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Calico ${CALICO_VERSION}"
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" create -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
fi

holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=available --timeout=300s deployment/tigera-operator -n tigera-operator

if ! kubectl --kubeconfig "$KUBECONFIG" get installations.operator.tigera.io default \
    -n tigera-operator &>/dev/null; then
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" apply -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml"
fi

holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-node -n calico-system
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=calico-kube-controllers -n calico-system

kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule- 2>/dev/null || true
kubectl label node --all node-role.kubernetes.io/worker= --overwrite 2>/dev/null || true
kubectl label node --all nvidia.com/holodeck.managed=true --overwrite 2>/dev/null || true

holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s nodes --all
holodeck_retry 20 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s pod -l k8s-app=kube-dns -n kube-system

sudo mkdir -p /etc/kubernetes
printf '%s\n' '{
  "source": "latest",
  "repo": "'"${GIT_REPO}"'",
  "branch": "'"${TRACK_BRANCH}"'",
  "commit": "'"${SHORT_COMMIT}"'",
  "version": "'"${K8S_VERSION}"'",
  "installer": "kubeadm",
  "installed_at": "'"$(date -Iseconds)"'"
}' | sudo tee /etc/kubernetes/PROVENANCE.json > /dev/null

holodeck_mark_installed "$COMPONENT" "${SHORT_COMMIT}"
holodeck_log "INFO" "$COMPONENT" "Successfully installed Kubernetes from ${TRACK_BRANCH}: ${SHORT_COMMIT}"
`

// Default Versions
const (
	defaultArch                  = "amd64"
	defaultKubernetesVersion     = "v1.33.3"
	defaultKubeletReleaseVersion = "v0.18.0"
	defaultCNIPluginsVersion     = "v1.7.1"
	defaultCRIVersion            = "v1.33.0"
	defaultCalicoVersion         = "v3.30.2"
)

// Kubernetes holds configuration for Kubernetes installation templates.
type Kubernetes struct {
	Version               string
	Installer             string
	KubeletReleaseVersion string
	Arch                  string
	CniPluginsVersion     string
	CalicoVersion         string
	CrictlVersion         string
	K8sEndpointHost       string
	K8sFeatureGates       string
	UseLegacyInit         bool
	CriSocket             string

	// Source configuration
	Source      string // "release", "git", "latest"
	GitRepo     string
	GitRef      string
	GitCommit   string // Resolved short SHA for git source
	TrackBranch string // For latest source

	// Kind exclusive
	KindConfig string
}

// KubeadmConfig holds configuration values for kubeadm
type KubeadmConfig struct {
	// Template defines the kubeadm configuration
	// template to use for generating the configuration
	// if no template is provided, the default template
	// is used
	Template string

	CriSocket            string
	ClusterName          string
	KubernetesVersion    string
	ControlPlaneEndpoint string
	PodSubnet            string
	FeatureGates         string // Feature gates as comma-separated string
	RuntimeConfig        string // Runtime config (for feature gates) resource.k8s.io/v1beta1=true
	IsUbuntu             bool   // Whether the system is Ubuntu (for resolvConf)
}

// NewKubernetes creates a Kubernetes template configuration from an Environment.
func NewKubernetes(env v1alpha1.Environment) (*Kubernetes, error) {
	kubernetes := &Kubernetes{}

	// Determine source (default to release)
	kubernetes.Source = string(env.Spec.Kubernetes.Source)
	if kubernetes.Source == "" {
		kubernetes.Source = "release"
	}

	// Configure based on source
	switch kubernetes.Source {
	case "git":
		if env.Spec.Kubernetes.Git != nil {
			kubernetes.GitRepo = env.Spec.Kubernetes.Git.Repo
			kubernetes.GitRef = env.Spec.Kubernetes.Git.Ref
		}
		if kubernetes.GitRepo == "" {
			kubernetes.GitRepo = "https://github.com/kubernetes/kubernetes.git"
		}

	case "latest":
		kubernetes.TrackBranch = "master"
		kubernetes.GitRepo = "https://github.com/kubernetes/kubernetes.git"
		if env.Spec.Kubernetes.Latest != nil {
			if env.Spec.Kubernetes.Latest.Track != "" {
				kubernetes.TrackBranch = env.Spec.Kubernetes.Latest.Track
			}
			if env.Spec.Kubernetes.Latest.Repo != "" {
				kubernetes.GitRepo = env.Spec.Kubernetes.Latest.Repo
			}
		}

	default: // "release"
		// Normalize Kubernetes version: always ensure it starts with 'v'
		// Check Release.Version first, then legacy KubernetesVersion
		version := ""
		if env.Spec.Kubernetes.Release != nil && env.Spec.Kubernetes.Release.Version != "" {
			version = env.Spec.Kubernetes.Release.Version
		} else if env.Spec.Kubernetes.KubernetesVersion != "" {
			version = env.Spec.Kubernetes.KubernetesVersion
		}

		switch {
		case version == "":
			kubernetes.Version = defaultKubernetesVersion
		case !strings.HasPrefix(version, "v"):
			kubernetes.Version = "v" + version
		default:
			kubernetes.Version = version
		}

		// Check if we need to use legacy mode (only for release)
		kubernetes.UseLegacyInit = isLegacyKubernetesVersion(kubernetes.Version)
	}

	// Common configuration
	if env.Spec.Kubernetes.KubeletReleaseVersion != "" {
		kubernetes.KubeletReleaseVersion = env.Spec.Kubernetes.KubeletReleaseVersion
	} else {
		kubernetes.KubeletReleaseVersion = defaultKubeletReleaseVersion
	}
	if env.Spec.Kubernetes.Arch != "" {
		kubernetes.Arch = env.Spec.Kubernetes.Arch
	} else {
		kubernetes.Arch = "amd64"
	}
	if env.Spec.Kubernetes.CniPluginsVersion != "" {
		kubernetes.CniPluginsVersion = env.Spec.Kubernetes.CniPluginsVersion
	} else {
		kubernetes.CniPluginsVersion = defaultCNIPluginsVersion
	}
	if env.Spec.Kubernetes.CalicoVersion != "" {
		kubernetes.CalicoVersion = env.Spec.Kubernetes.CalicoVersion
	} else {
		kubernetes.CalicoVersion = defaultCalicoVersion
	}
	if env.Spec.Kubernetes.CrictlVersion != "" {
		kubernetes.CrictlVersion = env.Spec.Kubernetes.CrictlVersion
	} else {
		kubernetes.CrictlVersion = defaultCRIVersion
	}
	if env.Spec.Kubernetes.K8sEndpointHost != "" {
		kubernetes.K8sEndpointHost = env.Spec.Kubernetes.K8sEndpointHost
	}
	if env.Spec.Kubernetes.K8sFeatureGates != nil {
		kubernetes.K8sFeatureGates = strings.Join(env.Spec.Kubernetes.K8sFeatureGates, ",")
	}
	if env.Spec.Kubernetes.KindConfig != "" {
		kubernetes.KindConfig = env.Spec.Kubernetes.KindConfig
	}

	// Get CRI socket path
	criSocket, err := GetCRISocket(string(env.Spec.ContainerRuntime.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to get CRI socket: %w", err)
	}
	kubernetes.CriSocket = criSocket

	return kubernetes, nil
}

// SetResolvedCommit sets the resolved git commit SHA.
func (k *Kubernetes) SetResolvedCommit(shortSHA string) {
	k.GitCommit = shortSHA
}

// Execute renders the appropriate Kubernetes template based on installer and
// source.
func (k *Kubernetes) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	var templateContent string
	var templateName string

	installer := env.Spec.Kubernetes.KubernetesInstaller
	if installer == "" {
		installer = "kubeadm"
	}

	switch installer {
	case "kubeadm":
		// Select template based on source
		switch k.Source {
		case "git":
			templateName = "kubeadm-git"
			templateContent = kubeadmGitTemplate
		case "latest":
			templateName = "kubeadm-latest"
			templateContent = kubeadmLatestTemplate
		default: // "release"
			templateName = "kubeadm"
			templateContent = KubeadmTemplate
		}

	case "kind":
		// Select template based on source
		switch k.Source {
		case "git":
			templateName = "kind-git"
			templateContent = kindGitTemplate
		case "latest":
			templateName = "kind-latest"
			templateContent = kindLatestTemplate
		default: // "release"
			templateName = "kind"
			templateContent = KindTemplate
		}

	case "microk8s":
		// MicroK8s only supports release source (validated earlier)
		templateName = "microk8s"
		templateContent = microk8sTemplate

	default:
		return fmt.Errorf("unknown kubernetes installer: %s", installer)
	}

	kubernetesTemplate := template.Must(
		template.New(templateName).Parse(templateContent),
	)

	if err := kubernetesTemplate.Execute(tpl, k); err != nil {
		return fmt.Errorf("failed to execute kubernetes template: %v", err)
	}

	return nil
}

// NewKubeadmConfig initializes a KubeadmConfig from a Kubernetes struct
func NewKubeadmConfig(env v1alpha1.Environment) (*KubeadmConfig, error) {
	// Convert feature gates slice into a comma-separated string
	featureGates := strings.Join(env.Spec.Kubernetes.K8sFeatureGates, ",")

	criSocket, err := GetCRISocket(string(env.Spec.ContainerRuntime.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to get CRI socket: %w", err)
	}

	kConfig := &KubeadmConfig{
		CriSocket:            criSocket, // Assuming containerd as default
		ClusterName:          "holodeck-cluster",
		KubernetesVersion:    env.Spec.Kubernetes.KubernetesVersion, // Uses provided Kubernetes version
		ControlPlaneEndpoint: env.Spec.Kubernetes.K8sEndpointHost,
		PodSubnet:            "192.168.0.0/16",               // Default subnet, modify if needed
		FeatureGates:         featureGates,                   // Convert slice to string for kubeadm
		RuntimeConfig:        "resource.k8s.io/v1beta1=true", // Example runtime config
		IsUbuntu:             true,                           // Default to true for Ubuntu-based deployments
	}

	if env.Spec.Kubernetes.KubernetesVersion == "" {
		kConfig.KubernetesVersion = defaultKubernetesVersion
	}

	if env.Spec.Kubernetes.KubeAdmConfig != "" {
		// open local file for reading
		// first check if file path is relative or absolute
		// if relative, then prepend the current working directory
		kubeAdmConfigPath := env.Spec.Kubernetes.KubeAdmConfig
		if !filepath.IsAbs(kubeAdmConfigPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return &KubeadmConfig{}, fmt.Errorf("failed to get current working directory: %v", err)
			}

			kubeAdmConfigPath = filepath.Join(cwd, strings.TrimPrefix(env.Spec.Kubernetes.KubeAdmConfig, "./"))
		}
		data, err := os.ReadFile(kubeAdmConfigPath) // nolint:gosec
		if err != nil {
			return &KubeadmConfig{}, fmt.Errorf("failed to read kubeadm config file: %v", err)
		}

		kConfig.Template = string(data)

	} else {
		kConfig.Template = kubeadmTemplate
	}

	return kConfig, nil
}

// ParseFeatureGates converts "Feature1=true,Feature2=false" to a map of strings for YAML
func (c *KubeadmConfig) ParseFeatureGates() map[string]string {
	parsed := make(map[string]string)
	if c.FeatureGates == "" {
		return parsed
	}
	// Split "FeatureX=true,FeatureY=false" into a map
	for kv := range strings.SplitSeq(c.FeatureGates, ",") {
		parts := strings.Split(kv, "=")
		if len(parts) == 2 {
			parsed[parts[0]] = parts[1] // Store values as "true"/"false" strings
		}
	}
	return parsed
}

// GenerateKubeadmConfig generates the kubeadm YAML configuration
func (c *KubeadmConfig) GenerateKubeadmConfig() (string, error) {
	// Parse feature gates into a structured map for YAML formatting
	data := struct {
		*KubeadmConfig
		ParsedFeatureGates map[string]string
	}{
		KubeadmConfig:      c,
		ParsedFeatureGates: c.ParseFeatureGates(),
	}

	// Parse and execute the template
	tmpl, err := template.New("kubeadmConfig").Parse(c.Template)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return output.String(), nil
}

// GetCRISocket returns the CRI socket path based on the container runtime
func GetCRISocket(runtime string) (string, error) {
	switch runtime {
	case "docker":
		return "unix:///run/cri-dockerd.sock", nil
	case "containerd":
		return "unix:///run/containerd/containerd.sock", nil
	case "crio":
		return "unix:///run/crio/crio.sock", nil
	default:
		return "", fmt.Errorf("unsupported container runtime: %s", runtime)
	}
}

// isLegacyKubernetesVersion checks if the Kubernetes version is older than v1.32.0
// which requires using legacy kubeadm init flags instead of config file
func isLegacyKubernetesVersion(version string) bool {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split version into components
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}

	// Parse major and minor versions
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])

	// Return true if version is older than v1.32.0
	return major < 1 || (major == 1 && minor < 32)
}
