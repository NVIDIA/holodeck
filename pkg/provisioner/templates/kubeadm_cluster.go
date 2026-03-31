/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

// KubeadmInitTemplate is the bash script for initializing the first control-plane node
const KubeadmInitTemplate = `
COMPONENT="kubernetes-kubeadm-init"
K8S_VERSION="{{.Version}}"
CNI_PLUGINS_VERSION="{{.CniPluginsVersion}}"
CALICO_VERSION="{{.CalicoVersion}}"
CRICTL_VERSION="{{.CrictlVersion}}"
{{if .Arch}}ARCH="{{.Arch}}"{{else}}ARCH="$(dpkg --print-architecture 2>/dev/null || (uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'))"{{end}}
KUBELET_RELEASE_VERSION="{{.KubeletReleaseVersion}}"
CONTROL_PLANE_ENDPOINT="{{.ControlPlaneEndpoint}}"
IS_HA="{{.IsHA}}"

holodeck_progress "$COMPONENT" 1 8 "Checking existing installation"

# Check if Kubernetes is already initialized
if [[ -f /etc/kubernetes/admin.conf ]]; then
    if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes &>/dev/null; then
        holodeck_log "INFO" "$COMPONENT" "Kubernetes cluster already initialized"
        holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
        exit 0
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

holodeck_progress "$COMPONENT" 4 8 "Installing Kubernetes binaries"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Install crictl (idempotent)
if [[ ! -f "$DOWNLOAD_DIR/crictl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing crictl ${CRICTL_VERSION}"
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | \
        sudo tar -C "$DOWNLOAD_DIR" -xz
fi

# Install kubeadm, kubelet, kubectl (idempotent)
if [[ ! -f "$DOWNLOAD_DIR/kubeadm" ]] || [[ ! -f "$DOWNLOAD_DIR/kubelet" ]] || \
   [[ ! -f "$DOWNLOAD_DIR/kubectl" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Installing kubeadm, kubelet, kubectl ${K8S_VERSION}"
    cd "$DOWNLOAD_DIR"
    sudo curl -L --remote-name-all \
        "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/{kubeadm,kubelet,kubectl}"
    sudo chmod +x kubeadm kubelet kubectl
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

# Detect this node's private IP for API server binding.
# Must be outside the init guard — used by verification and NLB switch below.
NODE_PRIVATE_IP=$(hostname -I | awk '{print $1}')

# Always use local IP for init health checks: kubeadm v1.33+ validates the API
# server via control-plane-endpoint, which may not be routable from within the
# instance during init (public IPs, NLB DNS, etc.). Use private IP for init and
# include the original endpoint in cert SANs so external access works.
if [[ "$CONTROL_PLANE_ENDPOINT" != "$NODE_PRIVATE_IP" ]]; then
    INIT_ENDPOINT="${NODE_PRIVATE_IP}"
    holodeck_log "INFO" "$COMPONENT" "Using local IP ${NODE_PRIVATE_IP} for init (endpoint: ${CONTROL_PLANE_ENDPOINT} in cert SANs)"
else
    INIT_ENDPOINT="${CONTROL_PLANE_ENDPOINT}"
fi

# Initialize cluster
if [[ ! -f /etc/kubernetes/admin.conf ]]; then
    INIT_ARGS=(
        --kubernetes-version="${K8S_VERSION}"
        --pod-network-cidr=192.168.0.0/16
        --control-plane-endpoint="${INIT_ENDPOINT}:6443"
        --apiserver-advertise-address="${NODE_PRIVATE_IP}"
        --apiserver-cert-extra-sans="${CONTROL_PLANE_ENDPOINT},${NODE_PRIVATE_IP},${INIT_ENDPOINT}"
        --ignore-preflight-errors=all
    )

    # For HA, upload certs so other control-plane nodes can join
    if [[ "$IS_HA" == "true" ]]; then
        INIT_ARGS+=(--upload-certs)
    fi

    holodeck_log "INFO" "$COMPONENT" "Running kubeadm init with args: ${INIT_ARGS[*]}"
    holodeck_retry 3 "$COMPONENT" sudo kubeadm init "${INIT_ARGS[@]}"

fi

# Setup kubeconfig (still points at local IP for HA; NLB switch happens after verification)
mkdir -p "$HOME/.kube"
sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="${HOME}/.kube/config"

holodeck_progress "$COMPONENT" 6 8 "Waiting for API server"

# Verify API server against local private IP first. For HA clusters, admin.conf
# still points at the local IP at this stage. For non-HA clusters this is a no-op
# since KUBECONFIG already targets the right endpoint.
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" \
    --server="https://${NODE_PRIVATE_IP}:6443" version

holodeck_progress "$COMPONENT" 7 8 "Installing Calico CNI"

# Install Calico (idempotent)
if ! kubectl --kubeconfig "$KUBECONFIG" get namespace tigera-operator &>/dev/null; then
    holodeck_log "INFO" "$COMPONENT" "Installing Calico ${CALICO_VERSION}"
    holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" create -f \
        "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
else
    holodeck_log "INFO" "$COMPONENT" "Tigera operator already installed"
fi

# Patch Tigera operator to use host networking and reach the API server directly.
# Without CNI, pods cannot reach the Kubernetes API server via cluster IP
# (10.96.0.1:443) because kube-proxy iptables rules may not be functional yet.
# The operator IS the CNI installer, so it must bypass cluster networking entirely.
# - hostNetwork: true — use the node's network stack
# - KUBERNETES_SERVICE_HOST=<node-ip> — reach API server via the node's IP
#   (must match a SAN in the kubeadm TLS cert; localhost is NOT in SANs)
# - KUBERNETES_SERVICE_PORT=6443 — use the real API server port, not the service port
NODE_IP=$(hostname -I | awk '{print $1}')
holodeck_log "INFO" "$COMPONENT" "Patching Tigera operator for host networking (API: ${NODE_IP}:6443)"
holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" patch deployment \
    tigera-operator -n tigera-operator --type=strategic -p "{
    \"spec\": {\"template\": {\"spec\": {
        \"hostNetwork\": true,
        \"dnsPolicy\": \"ClusterFirstWithHostNet\"
    }}}
}"
holodeck_retry 3 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" set env \
    deployment/tigera-operator -n tigera-operator \
    KUBERNETES_SERVICE_HOST="${NODE_IP}" KUBERNETES_SERVICE_PORT="6443"

# Wait for the patched rollout to complete
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" rollout status \
    deployment/tigera-operator -n tigera-operator --timeout=300s

# Wait for Tigera operator CRDs to be established before applying custom resources.
# The operator deployment becomes "available" before it has registered all its CRDs
# (Installation, APIServer, etc.), causing "no matches for kind" errors.
holodeck_log "INFO" "$COMPONENT" "Waiting for Tigera operator CRDs"
if ! holodeck_retry 30 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=established --timeout=10s crd/installations.operator.tigera.io; then
    # Diagnostic dump on failure
    holodeck_log "ERROR" "$COMPONENT" "CRD wait failed - collecting diagnostics"
    kubectl --kubeconfig "$KUBECONFIG" get pods -n tigera-operator -o wide 2>&1 || true
    kubectl --kubeconfig "$KUBECONFIG" describe pod -n tigera-operator 2>&1 | tail -40 || true
    kubectl --kubeconfig "$KUBECONFIG" logs -n tigera-operator -l name=tigera-operator --tail=30 2>&1 || true
    kubectl --kubeconfig "$KUBECONFIG" get events -n tigera-operator --sort-by='.lastTimestamp' 2>&1 | tail -20 || true
    kubectl --kubeconfig "$KUBECONFIG" get crd 2>&1 | grep -i tigera || true
    holodeck_error 6 "$COMPONENT" \
        "Tigera operator CRDs not registered after retries" \
        "The operator pod may be crashing. Check diagnostics above."
fi

# Calico v3.30.2+ custom-resources.yaml includes Goldmane and Whisker resources.
# Wait for their CRDs to be registered before applying, otherwise kubectl apply fails
# with "no matches for kind" for resources whose CRDs aren't established yet.
for crd in apiservers.operator.tigera.io goldmanes.operator.tigera.io whiskers.operator.tigera.io; do
    holodeck_retry 30 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
        --for=condition=established --timeout=10s "crd/${crd}" 2>/dev/null || \
        holodeck_log "WARN" "$COMPONENT" "CRD ${crd} not found — may not exist in this Calico version"
done

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

# For HA with NLB: now that Calico is running and the cluster is fully functional,
# switch the cluster config to use the NLB DNS so that join tokens reference the
# NLB endpoint (reachable by other nodes). This MUST happen after Calico — the NLB
# health checks require a working CNI to pass.
if [[ "$IS_HA" == "true" ]] && [[ "$INIT_ENDPOINT" != "$CONTROL_PLANE_ENDPOINT" ]]; then
    # Escape dots in INIT_ENDPOINT for safe sed regex matching (IPs contain literal dots)
    INIT_ESCAPED=$(echo "$INIT_ENDPOINT" | sed 's/\./\\./g')
    holodeck_log "INFO" "$COMPONENT" "Updating cluster config to use NLB endpoint: ${CONTROL_PLANE_ENDPOINT}:6443"
    # Update the kubeadm-config ConfigMap
    sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf -n kube-system get configmap kubeadm-config -o yaml | \
        sed "s|controlPlaneEndpoint: ${INIT_ESCAPED}:6443|controlPlaneEndpoint: ${CONTROL_PLANE_ENDPOINT}:6443|g" | \
        sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f - || \
        holodeck_log "WARN" "$COMPONENT" "Could not update kubeadm-config, join may need manual endpoint"
    # NOTE: Do NOT patch admin.conf to use the NLB endpoint.
    # CP nodes must use their local API server (localhost:6443) to avoid
    # AWS NLB hairpin routing — NLBs drop traffic when a registered target
    # connects through the NLB and gets routed back to itself.
fi

# Label this node as control-plane (keep the taint for multinode)
kubectl label node --all nvidia.com/holodeck.managed=true --overwrite 2>/dev/null || true

# Wait for this node to be ready
holodeck_log "INFO" "$COMPONENT" "Waiting for control-plane node"
holodeck_retry 10 "$COMPONENT" kubectl --kubeconfig "$KUBECONFIG" wait \
    --for=condition=ready --timeout=300s nodes --all

holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
holodeck_log "INFO" "$COMPONENT" "Control-plane initialized successfully"
`

// KubeadmJoinTemplate is the bash script for joining nodes to the cluster
const KubeadmJoinTemplate = `
COMPONENT="kubernetes-kubeadm-join"
CONTROL_PLANE_ENDPOINT="{{.ControlPlaneEndpoint}}"
TOKEN="{{.Token}}"
CA_CERT_HASH="{{.CACertHash}}"
IS_CONTROL_PLANE="{{.IsControlPlane}}"
{{- if .CertificateKey }}
CERTIFICATE_KEY="{{.CertificateKey}}"
{{- end }}

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if already joined
if [[ -f /etc/kubernetes/kubelet.conf ]]; then
    holodeck_log "INFO" "$COMPONENT" "Node already joined to cluster"
    holodeck_mark_installed "$COMPONENT" "joined"
    exit 0
fi

holodeck_progress "$COMPONENT" 2 4 "Configuring system prerequisites"

# Disable swap
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Configure modules
if [[ ! -f /etc/modules-load.d/k8s.conf ]]; then
    sudo tee /etc/modules-load.d/k8s.conf > /dev/null <<EOF
overlay
br_netfilter
EOF
fi
sudo modprobe overlay
sudo modprobe br_netfilter

# Set up sysctl params
if [[ ! -f /etc/sysctl.d/kubernetes.conf ]]; then
    sudo tee /etc/sysctl.d/kubernetes.conf > /dev/null <<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
    sudo sysctl --system
fi

holodeck_progress "$COMPONENT" 3 4 "Joining cluster"

# Build join command
JOIN_ARGS=(
    "${CONTROL_PLANE_ENDPOINT}:6443"
    --token "${TOKEN}"
    --discovery-token-ca-cert-hash "${CA_CERT_HASH}"
    --ignore-preflight-errors=all
)

if [[ "$IS_CONTROL_PLANE" == "true" ]]; then
    JOIN_ARGS+=(--control-plane)
    {{- if .CertificateKey }}
    JOIN_ARGS+=(--certificate-key "${CERTIFICATE_KEY}")
    {{- end }}
    holodeck_log "INFO" "$COMPONENT" "Joining as control-plane node"
else
    holodeck_log "INFO" "$COMPONENT" "Joining as worker node"
fi

holodeck_retry 3 "$COMPONENT" sudo kubeadm join "${JOIN_ARGS[@]}"

holodeck_progress "$COMPONENT" 4 4 "Configuring node"

# Setup kubeconfig for control-plane nodes
if [[ "$IS_CONTROL_PLANE" == "true" ]]; then
    # Patch admin.conf to use the local API server instead of the NLB endpoint.
    # AWS NLBs drop hairpin traffic (target connects through NLB back to itself),
    # so CP nodes must talk to their own local kube-apiserver.
    sudo sed -i 's|server: https://.*:6443|server: https://localhost:6443|g' \
        /etc/kubernetes/admin.conf
    mkdir -p "$HOME/.kube"
    sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
    sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
fi

holodeck_mark_installed "$COMPONENT" "joined"
holodeck_log "INFO" "$COMPONENT" "Node joined successfully"
`

var (
	kubeadmInitTmpl   = template.Must(template.New("kubeadm-init").Parse(KubeadmInitTemplate))
	kubeadmJoinTmpl   = template.Must(template.New("kubeadm-join").Parse(KubeadmJoinTemplate))
	kubeadmPrereqTmpl = template.Must(template.New("kubeadm-prereq").Parse(strings.TrimSpace(KubeadmPrereqTemplate)))
)

// KubeadmInitConfig holds configuration for kubeadm init
type KubeadmInitConfig struct {
	Environment          *v1alpha1.Environment
	ControlPlaneEndpoint string
	IsHA                 bool
}

// Execute generates the kubeadm init script
func (c *KubeadmInitConfig) Execute(tpl *bytes.Buffer) error {
	k, err := NewKubernetes(*c.Environment)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	data := struct {
		*Kubernetes
		ControlPlaneEndpoint string
		IsHA                 string
	}{
		Kubernetes:           k,
		ControlPlaneEndpoint: c.ControlPlaneEndpoint,
		IsHA:                 fmt.Sprintf("%t", c.IsHA),
	}

	if err := kubeadmInitTmpl.Execute(tpl, data); err != nil {
		return fmt.Errorf("failed to execute kubeadm init template: %w", err)
	}
	return nil
}

// KubeadmJoinConfig holds configuration for kubeadm join
type KubeadmJoinConfig struct {
	ControlPlaneEndpoint string
	Token                string
	CACertHash           string
	CertificateKey       string // Only for control-plane joins
	IsControlPlane       bool
}

// Execute generates the kubeadm join script
func (c *KubeadmJoinConfig) Execute(tpl *bytes.Buffer) error {
	data := struct {
		ControlPlaneEndpoint string
		Token                string
		CACertHash           string
		CertificateKey       string
		IsControlPlane       string
	}{
		ControlPlaneEndpoint: c.ControlPlaneEndpoint,
		Token:                c.Token,
		CACertHash:           c.CACertHash,
		CertificateKey:       c.CertificateKey,
		IsControlPlane:       fmt.Sprintf("%t", c.IsControlPlane),
	}

	if err := kubeadmJoinTmpl.Execute(tpl, data); err != nil {
		return fmt.Errorf("failed to execute kubeadm join template: %w", err)
	}
	return nil
}

// KubeadmPrereqTemplate is the bash script for installing Kubernetes binaries only
// This is used to prepare nodes before the actual init/join
const KubeadmPrereqTemplate = `
COMPONENT="kubernetes-prereq"
K8S_VERSION="{{.Version}}"
CNI_PLUGINS_VERSION="{{.CniPluginsVersion}}"
CRICTL_VERSION="{{.CrictlVersion}}"
{{if .Arch}}ARCH="{{.Arch}}"{{else}}ARCH="$(dpkg --print-architecture 2>/dev/null || (uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'))"{{end}}
KUBELET_RELEASE_VERSION="{{.KubeletReleaseVersion}}"

holodeck_progress "$COMPONENT" 1 3 "Checking existing installation"

# Check if already installed
if [[ -f /usr/local/bin/kubeadm ]] && [[ -f /usr/local/bin/kubelet ]]; then
    holodeck_log "INFO" "$COMPONENT" "Kubernetes binaries already installed"
    holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
    exit 0
fi

holodeck_progress "$COMPONENT" 2 3 "Installing CNI plugins"

DEST="/opt/cni/bin"
sudo mkdir -p "$DEST"
if [[ ! -f "$DEST/bridge" ]]; then
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | \
        sudo tar -C "$DEST" -xz
    sudo chmod -R 755 "$DEST"
fi

holodeck_progress "$COMPONENT" 3 3 "Installing Kubernetes binaries"

DOWNLOAD_DIR="/usr/local/bin"
sudo mkdir -p "$DOWNLOAD_DIR"

# Install crictl
if [[ ! -f "$DOWNLOAD_DIR/crictl" ]]; then
    holodeck_retry 3 "$COMPONENT" curl -L \
        "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | \
        sudo tar -C "$DOWNLOAD_DIR" -xz
fi

# Install kubeadm, kubelet, kubectl
if [[ ! -f "$DOWNLOAD_DIR/kubeadm" ]]; then
    cd "$DOWNLOAD_DIR"
    sudo curl -L --remote-name-all \
        "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/{kubeadm,kubelet,kubectl}"
    sudo chmod +x kubeadm kubelet kubectl
fi

# Configure kubelet service
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

holodeck_mark_installed "$COMPONENT" "$K8S_VERSION"
holodeck_log "INFO" "$COMPONENT" "Kubernetes binaries installed"
`

// KubeadmPrereqConfig holds configuration for installing K8s prerequisites
type KubeadmPrereqConfig struct {
	Environment *v1alpha1.Environment
}

// Execute generates the kubeadm prerequisites script
func (c *KubeadmPrereqConfig) Execute(tpl *bytes.Buffer) error {
	k, err := NewKubernetes(*c.Environment)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	if err := kubeadmPrereqTmpl.Execute(tpl, k); err != nil {
		return fmt.Errorf("failed to execute kubeadm prereq template: %w", err)
	}
	return nil
}
