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
	"text/template"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const defaultNVBranch = "575"

// From https://docs.nvidia.com/datacenter/tesla/tesla-installation-notes/index.html#ubuntu-lts
const NvDriverTemplate = `
COMPONENT="nvidia-driver"
DESIRED_VERSION="{{.Version}}"
DESIRED_BRANCH="{{.Branch}}"

# Check for NVIDIA GPU hardware before attempting installation
# This allows mixed CPU/GPU clusters to work correctly
holodeck_log "INFO" "$COMPONENT" "Checking for NVIDIA GPU hardware..."
if ! lspci 2>/dev/null | grep -qi 'nvidia\|3d controller'; then
    holodeck_log "INFO" "$COMPONENT" "No NVIDIA GPU detected on this node, skipping driver installation"
    exit 0
fi
holodeck_log "INFO" "$COMPONENT" "NVIDIA GPU detected, proceeding with driver installation"

holodeck_progress "$COMPONENT" 1 5 "Checking existing installation"

# Check if driver is already installed and functional
if command -v nvidia-smi &>/dev/null; then
    INSTALLED_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        # Check if version matches (if specified)
        if [[ -z "$DESIRED_VERSION" ]] || [[ "$INSTALLED_VERSION" == "$DESIRED_VERSION" ]]; then
            holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

            # Verify driver is functional
            if holodeck_verify_driver; then
                holodeck_log "INFO" "$COMPONENT" "Driver verified functional"
                holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                exit 0
            else
                holodeck_log "WARN" "$COMPONENT" \
                    "Driver installed but not functional, attempting repair"
            fi
        else
            holodeck_log "INFO" "$COMPONENT" \
                "Version mismatch: installed=${INSTALLED_VERSION}, desired=${DESIRED_VERSION}"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 5 "Installing dependencies"

# Check kernel headers availability BEFORE attempting install
KERNEL_VERSION=$(uname -r)
holodeck_log "INFO" "$COMPONENT" "Checking kernel headers for ${KERNEL_VERSION}"

holodeck_retry 3 "$COMPONENT" sudo apt-get update

if ! apt-cache show "linux-headers-${KERNEL_VERSION}" >/dev/null 2>&1; then
    holodeck_log "WARN" "$COMPONENT" \
        "Kernel headers for ${KERNEL_VERSION} not found in repositories"

    # Try to find a compatible kernel header package
    KERNEL_BASE=$(echo "${KERNEL_VERSION}" | cut -d- -f1-2)
    holodeck_log "INFO" "$COMPONENT" \
        "Searching for compatible headers with base version ${KERNEL_BASE}"
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | \
        grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')

    if [[ -n "$COMPATIBLE_HEADERS" ]]; then
        holodeck_log "WARN" "$COMPONENT" \
            "Using potentially compatible headers: $COMPATIBLE_HEADERS"
        holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$COMPATIBLE_HEADERS"
    else
        holodeck_error 4 "$COMPONENT" \
            "No compatible kernel headers found for ${KERNEL_VERSION}" \
            "Update kernel or use a different AMI with available headers"
    fi
else
    holodeck_retry 3 "$COMPONENT" install_packages_with_retry "linux-headers-${KERNEL_VERSION}"
fi

holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    apt-utils build-essential ca-certificates curl kmod file \
    libelf-dev libglvnd-dev pkg-config make

holodeck_retry 3 "$COMPONENT" install_packages_with_retry gcc-12 g++-12
sudo update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12
sudo update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

holodeck_progress "$COMPONENT" 3 5 "Adding CUDA repository"

# Add CUDA repository (idempotent - install if either list or keyring is missing)
if [[ ! -f /etc/apt/sources.list.d/cuda*.list ]] || \
   [[ ! -f /usr/share/keyrings/cuda-archive-keyring.gpg ]]; then
    distribution=$(. /etc/os-release; echo "${ID}${VERSION_ID}" | sed -e 's/\.//g')
    holodeck_retry 3 "$COMPONENT" wget -q \
        "https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb"
    sudo dpkg -i cuda-keyring_1.1-1_all.deb
    rm -f cuda-keyring_1.1-1_all.deb
    holodeck_retry 3 "$COMPONENT" sudo apt-get update
else
    holodeck_log "INFO" "$COMPONENT" "CUDA repository already configured"
fi

holodeck_progress "$COMPONENT" 4 5 "Installing NVIDIA driver"

# Install driver
DRIVER_PACKAGE="cuda-drivers"
if [[ -n "$DESIRED_VERSION" ]]; then
    DRIVER_PACKAGE="${DRIVER_PACKAGE}=${DESIRED_VERSION}"
elif [[ -n "$DESIRED_BRANCH" ]]; then
    DRIVER_PACKAGE="${DRIVER_PACKAGE}-${DESIRED_BRANCH}"
fi

holodeck_log "INFO" "$COMPONENT" "Installing package: ${DRIVER_PACKAGE}"
holodeck_retry 3 "$COMPONENT" install_packages_with_retry "$DRIVER_PACKAGE"

holodeck_progress "$COMPONENT" 5 5 "Verifying installation"

# Load module if not loaded
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia || holodeck_error 10 "$COMPONENT" \
        "Failed to load nvidia kernel module" \
        "Check dmesg for kernel module errors: dmesg | grep -i nvidia"
fi

# Start persistenced
sudo nvidia-persistenced --persistence-mode || true

# Final verification
if ! holodeck_verify_driver; then
    holodeck_error 5 "$COMPONENT" \
        "Driver installation verification failed" \
        "Run 'dmesg | grep -i nvidia' and 'nvidia-smi' to diagnose"
fi

FINAL_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" "Successfully installed driver version ${FINAL_VERSION}"
`

type NvDriver v1alpha1.NVIDIADriver

func NewNvDriver(env v1alpha1.Environment) *NvDriver {
	var nvDriver NvDriver

	// Propagate user-supplied settings
	nvDriver.Install = env.Spec.NVIDIADriver.Install
	nvDriver.Branch = env.Spec.NVIDIADriver.Branch
	nvDriver.Version = env.Spec.NVIDIADriver.Version

	// Apply default branch only when neither a specific version nor a branch was provided
	if nvDriver.Version == "" && nvDriver.Branch == "" {
		nvDriver.Branch = defaultNVBranch
	}

	return &nvDriver
}

func (t *NvDriver) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	nvDriverTemplate := template.Must(template.New("nv-driver").Parse(NvDriverTemplate))
	err := nvDriverTemplate.Execute(tpl, t)
	if err != nil {
		return fmt.Errorf("failed to execute nv-driver template: %v", err)
	}
	return nil
}
