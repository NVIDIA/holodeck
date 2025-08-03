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
// Based on the approach from holodeck v0.2.5 that worked
const containerdV1Template = `
: ${CONTAINERD_VERSION:={{.Version}}}

# Install containerd using the proven v0.2.5 approach
echo "Installing containerd {{.Version}} using apt repository..."

# Install required packages
with_retry 3 10s sudo apt-get update
install_packages_with_retry ca-certificates curl gnupg -y
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add the repository to Apt sources:
echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
with_retry 3 10s sudo apt-get update

# Install containerd with specific version if provided
if [ -n "{{.Version}}" ] && [ "{{.Version}}" != "latest" ]; then
    # Try to install specific version
    echo "Attempting to install containerd.io={{.Version}}-1..."
    if ! install_packages_with_retry containerd.io={{.Version}}-1; then
        echo "Specific version {{.Version}} not found, installing latest..."
        install_packages_with_retry containerd.io
    fi
else
    install_packages_with_retry containerd.io
fi

# Configure containerd and start service
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml
# Set systemd as the cgroup driver 
# see https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd
sudo sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
# This ensures containerd looks in the right places for CNI plugins
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# restart containerd
sudo systemctl restart containerd
sudo systemctl enable containerd

# Wait for containerd to be ready
timeout=30
while ! sudo ctr version &>/dev/null; do
    if [ $timeout -le 0 ]; then
        echo "Timeout waiting for containerd"
        exit 1
    fi
    sleep 1
    timeout=$((timeout-1))
done

# Verify installation
sudo ctr version
echo "Containerd installation completed!"
`

// containerdV2Template is used for containerd 2.x versions
// Based on official containerd installation guide for v2.x
const containerdV2Template = `
: ${CONTAINERD_VERSION:={{.Version}}}

# Install containerd {{.Version}} from official binaries
echo "Installing containerd {{.Version}} from official binaries..."

# Set up prerequisites
sudo modprobe overlay
sudo modprobe br_netfilter

# Setup required sysctl params
cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

# Apply sysctl params
sudo sysctl --system

# Install dependencies
with_retry 3 10s sudo apt-get update
install_packages_with_retry ca-certificates curl

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" ]]; then
    ARCH="arm64"
fi

# Download containerd
CONTAINERD_TAR="containerd-{{.Version}}-linux-${ARCH}.tar.gz"
CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v{{.Version}}/${CONTAINERD_TAR}"

echo "Downloading containerd from $CONTAINERD_URL"
with_retry 3 10s curl -fsSL -o ${CONTAINERD_TAR} ${CONTAINERD_URL}

# Extract containerd
sudo tar Cxzvf /usr/local ${CONTAINERD_TAR}
rm -f ${CONTAINERD_TAR}

# Download and install runc
RUNC_VERSION="1.2.3"  # Latest stable version as of Dec 2024
echo "Installing runc ${RUNC_VERSION}..."
with_retry 3 10s curl -fsSL -o runc.${ARCH} https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}
sudo install -m 755 runc.${ARCH} /usr/local/sbin/runc
rm -f runc.${ARCH}

# Install CNI plugins
CNI_VERSION="v1.6.2"  # Latest stable version as of Dec 2024
CNI_TAR="cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz"
echo "Installing CNI plugins ${CNI_VERSION}..."
with_retry 3 10s curl -fsSL -o ${CNI_TAR} https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/${CNI_TAR}
sudo mkdir -p /opt/cni/bin
sudo tar Cxzvf /opt/cni/bin ${CNI_TAR}
rm -f ${CNI_TAR}

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml

# Update config for systemd cgroup
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Ensure CNI paths are configured correctly
# This ensures containerd looks in the right places for CNI plugins
sudo sed -i 's|conf_dir = .*|conf_dir = "/etc/cni/net.d"|g' /etc/containerd/config.toml
sudo sed -i 's|bin_dir = .*|bin_dir = "/opt/cni/bin"|g' /etc/containerd/config.toml

# Create containerd service
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

# Start containerd
sudo systemctl daemon-reload
sudo systemctl enable --now containerd

# Wait for containerd to be ready
timeout=30
while ! sudo ctr version &>/dev/null; do
    if [ $timeout -le 0 ]; then
        echo "Timeout waiting for containerd"
        exit 1
    fi
    sleep 1
    timeout=$((timeout-1))
done

# Verify installation
sudo ctr version
echo "Containerd {{.Version}} (v2.x) installation completed!"
`

type Containerd struct {
	Version      string
	MajorVersion int
}

func NewContainerd(env v1alpha1.Environment) *Containerd {
	var version string

	if env.Spec.ContainerRuntime.Version == "" {
		version = "1.7.27" // Default to v1.7.x
	} else {
		// remove the 'v' prefix from the version if it exists
		version = strings.TrimPrefix(env.Spec.ContainerRuntime.Version, "v")
	}

	// Parse major version
	majorVersion := 1
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		if major := parts[0]; major == "2" {
			majorVersion = 2
			// If user specifies "2" without minor/patch, use latest stable v2
			if version == "2" {
				version = "2.0.0"
			}
		}
	}

	return &Containerd{
		Version:      version,
		MajorVersion: majorVersion,
	}
}

func (t *Containerd) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	// Choose template based on major version
	var templateContent string
	switch t.MajorVersion {
	case 2:
		templateContent = containerdV2Template
	default:
		templateContent = containerdV1Template
	}

	containerdTemplate := template.Must(template.New("containerd").Parse(templateContent))
	err := containerdTemplate.Execute(tpl, t)
	if err != nil {
		return fmt.Errorf("failed to execute containerd template: %v", err)
	}
	return nil
}
