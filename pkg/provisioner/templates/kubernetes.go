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

// Default Versions
const (
	defaultArch                  = "amd64"
	defaultKubernetesVersion     = "v1.33.3"
	defaultKubeletReleaseVersion = "v0.18.0"
	defaultCNIPluginsVersion     = "v1.7.1"
	defaultCRIVersion            = "v1.33.0"
	defaultCalicoVersion         = "v3.30.2"
)

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

func NewKubernetes(env v1alpha1.Environment) (*Kubernetes, error) {
	kubernetes := &Kubernetes{}

	// Normalize Kubernetes version: always ensure it starts with 'v'
	switch {
	case env.Spec.Kubernetes.KubernetesVersion == "":
		kubernetes.Version = defaultKubernetesVersion
	case !strings.HasPrefix(env.Spec.Kubernetes.KubernetesVersion, "v"):
		kubernetes.Version = "v" + env.Spec.Kubernetes.KubernetesVersion
	default:
		kubernetes.Version = env.Spec.Kubernetes.KubernetesVersion
	}
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

	// Check if we need to use legacy mode
	kubernetes.UseLegacyInit = isLegacyKubernetesVersion(kubernetes.Version)

	// Get CRI socket path
	criSocket, err := GetCRISocket(string(env.Spec.ContainerRuntime.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to get CRI socket: %w", err)
	}
	kubernetes.CriSocket = criSocket

	return kubernetes, nil
}

func (k *Kubernetes) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	kubernetesTemplate := new(template.Template)

	switch env.Spec.Kubernetes.KubernetesInstaller {
	case "kubeadm":
		kubernetesTemplate = template.Must(template.New("kubeadm").Parse(KubeadmTemplate))
	case "kind":
		kubernetesTemplate = template.Must(template.New("kind").Parse(KindTemplate))
	case "microk8s":
		kubernetesTemplate = template.Must(template.New("microk8s").Parse(microk8sTemplate))
	default:
		return fmt.Errorf("unknown kubernetes installer %s", env.Spec.Kubernetes.KubernetesInstaller)
	}

	err := kubernetesTemplate.Execute(tpl, k)
	if err != nil {
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
