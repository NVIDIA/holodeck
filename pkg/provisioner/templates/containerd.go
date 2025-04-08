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

# Install required packages
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

ARCH=$(uname -m)
CONTAINERD_TAR="containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz"
CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/${CONTAINERD_TAR}"
CONTAINERD_SHA256_URL="https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/SHA256SUMS"

echo "Downloading and extracting containerd ${CONTAINERD_VERSION} from: $CONTAINERD_URL"

# Download containerd tarball and checksum
curl -fsSL -o ${CONTAINERD_TAR} ${CONTAINERD_URL}
curl -fsSL -o containerd_SHA256SUMS ${CONTAINERD_SHA256_URL}

# Verify SHA256 checksum
EXPECTED_CHECKSUM=$(grep ${CONTAINERD_TAR} containerd_SHA256SUMS | awk '{print $1}')
ACTUAL_CHECKSUM=$(sha256sum ${CONTAINERD_TAR} | awk '{print $1}')

if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Checksum verification failed for containerd. Exiting."
    exit 1
fi

# Stream directly into tar to avoid saving the archive
cat ${CONTAINERD_TAR} | sudo tar Cxzvf /usr/local -

# Cleanup
rm -f ${CONTAINERD_TAR} containerd_SHA256SUMS

echo "Containerd ${CONTAINERD_VERSION} installed successfully."

# Fetch latest stable RUNC version from GitHub
echo "Fetching latest stable runc version..."
RUNC_VERSION=$(curl -fsSL https://api.github.com/repos/opencontainers/runc/releases/latest | grep '"tag_name":' | cut -d '"' -f 4 | sed 's/v//')

if [ -z "$RUNC_VERSION" ]; then
    echo "Failed to fetch latest RUNC version. Using default version."
    RUNC_VERSION="1.2.6"
fi

RUNC_URL="https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${ARCH}"
RUNC_SHA256_URL="https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/SHA256SUMS"

echo "Downloading runc ${RUNC_VERSION} from: $RUNC_URL"

# Download runc binary and checksum
curl -fsSL -o runc.${ARCH} ${RUNC_URL}
curl -fsSL -o SHA256SUMS ${RUNC_SHA256_URL}

# Verify SHA256 checksum
EXPECTED_CHECKSUM=$(grep "runc.${ARCH}" SHA256SUMS | awk '{print $1}')
ACTUAL_CHECKSUM=$(sha256sum runc.${ARCH} | awk '{print $1}')

if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Checksum verification failed for runc. Exiting."
    exit 1
fi

sudo install -m 755 runc.${ARCH} /usr/local/sbin/runc
rm -f runc.${ARCH} SHA256SUMS

echo "Runc ${RUNC_VERSION} installed successfully."

# Install CNI plugins
CNI_VERSION="1.1.1"
CNI_TAR="cni-plugins-linux-${ARCH}-v${CNI_VERSION}.tgz"
CNI_URL="https://github.com/containernetworking/plugins/releases/download/v${CNI_VERSION}/${CNI_TAR}"
CNI_SHA256_URL="https://github.com/containernetworking/plugins/releases/download/v${CNI_VERSION}/SHA256SUMS"

echo "Downloading CNI plugins from: $CNI_URL"

# Download CNI tarball and checksum
curl -fsSL -o ${CNI_TAR} ${CNI_URL}
curl -fsSL -o SHA256SUMS ${CNI_SHA256_URL}

# Verify SHA256 checksum
EXPECTED_CHECKSUM=$(grep ${CNI_TAR} SHA256SUMS | awk '{print $1}')
ACTUAL_CHECKSUM=$(sha256sum ${CNI_TAR} | awk '{print $1}')

if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Checksum verification failed for CNI plugins. Exiting."
    exit 1
fi

sudo mkdir -p /opt/cni/bin
sudo tar Cxzvf /opt/cni/bin ${CNI_TAR}
rm -f ${CNI_TAR} SHA256SUMS

# Configure containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml

# Set systemd as the cgroup driver
# see https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Set up systemd service for containerd
sudo curl -fsSL "https://raw.githubusercontent.com/containerd/containerd/main/containerd.service" -o /etc/systemd/system/containerd.service
sudo systemctl daemon-reload
sudo systemctl enable --now containerd

# Verify installation
containerd --version
runc --version
echo "Containerd installation completed!"
`

type Containerd struct {
	Version string
}

func NewContainerd(env v1alpha1.Environment) *Containerd {
	var version string

	if env.Spec.ContainerRuntime.Version == "" {
		version = "1.6.27"
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
