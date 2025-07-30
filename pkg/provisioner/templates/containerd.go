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

const containerdTemplate = `
: ${CONTAINERD_VERSION:={{.Version}}}

# Check system requirements
echo "Checking system requirements..."

# Check for systemd
if ! command -v systemctl &> /dev/null; then
    echo "Error: systemd is required but not installed"
    exit 1
fi

# Check and load required kernel modules
echo "Checking and loading required kernel modules..."
REQUIRED_MODULES="overlay br_netfilter"
for module in $REQUIRED_MODULES; do
    if ! lsmod | grep -q "^${module}"; then
        echo "Loading ${module} module..."
        if ! sudo modprobe ${module}; then
            echo "Error: Failed to load ${module} module"
            exit 1
        fi
    fi
done

# Ensure modules are loaded at boot
for module in $REQUIRED_MODULES; do
    if [ ! -f "/etc/modules-load.d/${module}.conf" ]; then
        echo "${module}" | sudo tee "/etc/modules-load.d/${module}.conf" > /dev/null
    fi
done

# Check and configure sysctl settings
echo "Configuring sysctl settings..."
SYSCTL_SETTINGS=(
    "net.bridge.bridge-nf-call-iptables=1"
    "net.bridge.bridge-nf-call-ip6tables=1"
    "net.ipv4.ip_forward=1"
)

for setting in "${SYSCTL_SETTINGS[@]}"; do
    key=$(echo $setting | cut -d= -f1)
    value=$(echo $setting | cut -d= -f2)
    if [ "$(sudo sysctl -n $key)" != "$value" ]; then
        echo "Setting $key to $value"
        sudo sysctl -w $key=$value
        echo "$key=$value" | sudo tee -a /etc/sysctl.conf > /dev/null
    fi
done

# Apply sysctl settings
sudo sysctl --system

# Check for required commands
REQUIRED_COMMANDS="curl tar systemctl"
for cmd in $REQUIRED_COMMANDS; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: Required command '$cmd' not found"
        exit 1
    fi
done

# Install required packages
echo "Installing required packages..."
with_retry 3 10s sudo apt-get update
install_packages_with_retry ca-certificates curl gnupg -y
sudo install -m 0755 -d /etc/apt/keyrings

# Check if CONTAINERD_VERSION is empty, if so fetch the latest stable version
if [ -z "$CONTAINERD_VERSION" ]; then
    echo "Fetching latest stable containerd version..."
    CONTAINERD_VERSION=$(curl -fsSL https://api.github.com/repos/containerd/containerd/releases/latest | grep '"tag_name":' | cut -d '"' -f 4 | sed 's/v//')

    if [ -z "$CONTAINERD_VERSION" ]; then
        echo "Failed to fetch latest Containerd version. Exiting."
        exit 1
    fi
fi
echo "Using containerd version: $CONTAINERD_VERSION"

# Determine major version for configuration
MAJOR_VERSION=$(echo $CONTAINERD_VERSION | cut -d. -f1)

# Check architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" ]]; then
    ARCH="arm64"
else
    echo "Error: Unsupported architecture: $ARCH"
    exit 1
fi

CONTAINERD_TAR="containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz"
CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/${CONTAINERD_TAR}"
CONTAINERD_SHA256_URL="${CONTAINERD_URL}.sha256sum"

echo "Downloading and extracting containerd ${CONTAINERD_VERSION} from: $CONTAINERD_URL"

# Create temporary directory for downloads
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

# Download containerd tarball and checksum
if ! curl -fsSL -o ${CONTAINERD_TAR} ${CONTAINERD_URL}; then
    echo "Error: Failed to download containerd tarball"
    exit 1
fi

if ! curl -fsSL -o containerd_SHA256SUMS ${CONTAINERD_SHA256_URL}; then
    echo "Error: Failed to download containerd checksum"
    exit 1
fi

# Verify SHA256 checksum
EXPECTED_CHECKSUM=$(cat containerd_SHA256SUMS | awk '{print $1}')
ACTUAL_CHECKSUM=$(sha256sum ${CONTAINERD_TAR} | awk '{print $1}')

if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Error: Checksum verification failed for containerd"
    echo "Expected: $EXPECTED_CHECKSUM"
    echo "Actual:   $ACTUAL_CHECKSUM"
    exit 1
fi

# Stream directly into tar to avoid saving the archive
if ! cat ${CONTAINERD_TAR} | sudo tar Cxzvf /usr/local -; then
    echo "Error: Failed to extract containerd tarball"
    exit 1
fi

# Cleanup
cd - > /dev/null
rm -rf $TMP_DIR

echo "Containerd ${CONTAINERD_VERSION} installed successfully."

# Fetch latest stable RUNC version from GitHub
echo "Fetching latest stable runc version..."
RUNC_VERSION=$(curl -fsSL https://api.github.com/repos/opencontainers/runc/releases/latest | grep '"tag_name":' | cut -d '"' -f 4 | sed 's/v//')

if [ -z "$RUNC_VERSION" ]; then
    echo "Failed to fetch latest RUNC version. Using default version."
    RUNC_VERSION="1.3.0"
fi

RUNC_URL="https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"

echo "Downloading runc ${RUNC_VERSION} from: $RUNC_URL"

# Download runc binary and checksum
curl -fsSL -o runc.${ARCH} ${RUNC_URL}

sudo install -m 755 runc.${ARCH} /usr/local/sbin/runc

echo "Runc ${RUNC_VERSION} installed successfully."

# Install CNI plugins
CNI_VERSION="1.3.0"
CNI_TAR="cni-plugins-linux-${ARCH}-v${CNI_VERSION}.tgz"
CNI_URL="https://github.com/containernetworking/plugins/releases/download/v${CNI_VERSION}/${CNI_TAR}"

echo "Downloading CNI plugins from: $CNI_URL"

# Download CNI tarball and checksum
curl -fsSL -o ${CNI_TAR} ${CNI_URL}

sudo mkdir -p /opt/cni/bin
sudo tar Cxzvf /opt/cni/bin ${CNI_TAR}

# Configure containerd
sudo mkdir -p /etc/containerd

# Create unified configuration that works for both 1.x and 2.x
# Start with a minimal config and add only what's needed
cat <<'EOF' | sudo tee /etc/containerd/config.toml > /dev/null
# /etc/containerd/config.toml (managed by Holodeck)
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.k8s.io/pause:3.9"
    [plugins."io.containerd.grpc.v1.cri".cni]
      # Include both locations to survive distro variance
      bin_dir = "/opt/cni/bin:/usr/libexec/cni"
      conf_dir = "/etc/cni/net.d"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]

[grpc]
  address = "/run/containerd/containerd.sock"
EOF

# Ensure CNI directories exist
sudo mkdir -p /etc/cni/net.d
sudo mkdir -p /opt/cni/bin

# Ensure containerd directories exist
sudo mkdir -p /var/lib/containerd
sudo mkdir -p /run/containerd

# Set up systemd service for containerd
sudo curl -fsSL "https://raw.githubusercontent.com/containerd/containerd/main/containerd.service" -o /etc/systemd/system/containerd.service

# Create containerd service directory
sudo mkdir -p /etc/systemd/system/containerd.service.d

# Add custom service configuration
cat <<EOF | sudo tee /etc/systemd/system/containerd.service.d/override.conf > /dev/null
[Service]
ExecStart=
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
# Ensure socket directory exists with correct permissions
ExecStartPre=/bin/mkdir -p /run/containerd
ExecStartPre=/bin/chmod 711 /run/containerd
EOF

# Ensure containerd is not running with stale config
sudo systemctl stop containerd || true

# Reload systemd and start containerd
sudo systemctl daemon-reload
echo "Starting containerd service..."
if ! sudo systemctl enable --now containerd; then
    echo "ERROR: Failed to start containerd service"
    echo "Checking service status..."
    sudo systemctl status containerd || true
    echo "Checking journal logs..."
    sudo journalctl -xeu containerd -n 50 || true
    echo "Checking config file syntax..."
    sudo containerd config dump || true
    exit 1
fi

# Wait for containerd to be ready
timeout=60
while ! sudo ctr version &>/dev/null; do
    if [ $timeout -le 0 ]; then
        echo "Timeout waiting for containerd to be ready"
        exit 1
    fi
    sleep 1
    timeout=$((timeout-1))
done

# Ensure socket permissions are correct
sudo chmod 666 /run/containerd/containerd.sock

# Verify installation
containerd --version
runc --version
sudo ctr version

# Verify CNI configuration
echo "Verifying containerd CNI configuration..."
if ! sudo grep -q 'bin_dir = "/opt/cni/bin:/usr/libexec/cni"' /etc/containerd/config.toml; then
    echo "ERROR: CNI bin_dir not properly configured in containerd"
    exit 1
fi

if ! sudo grep -q 'conf_dir = "/etc/cni/net.d"' /etc/containerd/config.toml; then
    echo "ERROR: CNI conf_dir not properly configured in containerd"
    exit 1
fi

if ! sudo grep -q 'SystemdCgroup = true' /etc/containerd/config.toml; then
    echo "ERROR: SystemdCgroup not enabled in containerd config"
    exit 1
fi

# Verify with crictl
if command -v crictl &> /dev/null; then
    echo "Checking CRI configuration..."
    sudo crictl info | grep -E "cni|Cni" || true
fi

# Note about nvidia-container-toolkit compatibility
echo ""
echo "Note: This containerd configuration is designed to be compatible with nvidia-container-toolkit."
echo "When nvidia-ctk runtime configure is run later, it will:"
echo "  - Add nvidia runtime configuration"
echo "  - Preserve our CNI settings (bin_dir and conf_dir)"
echo "  - May change default_runtime_name to 'nvidia'"
echo "This is expected and will not affect CNI functionality."

# Test containerd functionality
sudo ctr images pull docker.io/library/hello-world:latest
sudo ctr run --rm docker.io/library/hello-world:latest test

echo "Containerd installation and CNI configuration completed successfully!"
`

type Containerd struct {
	Version string
}

func NewContainerd(env v1alpha1.Environment) *Containerd {
	var version string

	if env.Spec.ContainerRuntime.Version == "" {
		version = "1.7.28"
	} else {
		// remove the 'v' prefix from the version if it exists
		version = strings.TrimPrefix(env.Spec.ContainerRuntime.Version, "v")
	}

	return &Containerd{
		Version: version,
	}
}

func (t *Containerd) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	containerdTemplate := template.Must(template.New("containerd").Parse(containerdTemplate))
	err := containerdTemplate.Execute(tpl, &Containerd{Version: t.Version})
	if err != nil {
		return fmt.Errorf("failed to execute containerd template: %v", err)
	}
	return nil
}
