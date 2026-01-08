/**
# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestNVDriverTemplate(t *testing.T) {
	testCases := []struct {
		description    string
		driver         *NvDriver
		expecteError   error
		expectedOutput string
	}{
		{
			description: "Version is set",
			driver: &NvDriver{
				Version: "123.4.5",
			},
			expectedOutput: `

# Install Dependencies
with_retry 3 10s sudo apt-get update

# Check if kernel headers are available for the current kernel
KERNEL_VERSION=$(uname -r)
echo "Checking for kernel headers for ${KERNEL_VERSION}..."

if ! apt-cache show linux-headers-${KERNEL_VERSION} >/dev/null 2>&1; then
    echo "ERROR: Kernel headers for ${KERNEL_VERSION} not found in repositories"
    echo "Available kernel headers:"
    apt-cache search linux-headers | grep -E "linux-headers-[0-9]" | sort
    
    # Try to find a compatible kernel header package
    KERNEL_BASE=$(echo ${KERNEL_VERSION} | cut -d- -f1-2)
    echo "Searching for compatible headers with base version ${KERNEL_BASE}..."
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')
    
    if [ -n "$COMPATIBLE_HEADERS" ]; then
        echo "WARNING: Using potentially compatible headers package: $COMPATIBLE_HEADERS"
        echo "This may cause issues with NVIDIA driver compilation"
        install_packages_with_retry $COMPATIBLE_HEADERS
    else
        echo "FATAL: No compatible kernel headers found. Cannot proceed with NVIDIA driver installation."
        exit 1
    fi
else
    install_packages_with_retry linux-headers-$(uname -r)
fi

install_packages_with_retry apt-utils build-essential ca-certificates curl kmod file libelf-dev libglvnd-dev pkg-config make

install_packages_with_retry gcc-12 g++-12 && \
    update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12 && \
    update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

# Install the new cuda-keyring package	
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb
with_retry 3 10s sudo apt-get update

# Install the NVIDIA driver
install_packages_with_retry cuda-drivers=123.4.5

# Check if NVIDIA module is loaded, if not load it
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia
fi

# Start nvidia-persistenced
sudo nvidia-persistenced --persistence-mode

# Quick check to see if the driver is installed
nvidia-smi
`,
		},
		{
			description: "Branch is set",
			driver: &NvDriver{
				Branch: "550",
			},
			expectedOutput: `
# Install Dependencies
with_retry 3 10s sudo apt-get update

# Check if kernel headers are available for the current kernel
KERNEL_VERSION=$(uname -r)
echo "Checking for kernel headers for ${KERNEL_VERSION}..."

if ! apt-cache show linux-headers-${KERNEL_VERSION} >/dev/null 2>&1; then
    echo "ERROR: Kernel headers for ${KERNEL_VERSION} not found in repositories"
    echo "Available kernel headers:"
    apt-cache search linux-headers | grep -E "linux-headers-[0-9]" | sort
    
    # Try to find a compatible kernel header package
    KERNEL_BASE=$(echo ${KERNEL_VERSION} | cut -d- -f1-2)
    echo "Searching for compatible headers with base version ${KERNEL_BASE}..."
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')
    
    if [ -n "$COMPATIBLE_HEADERS" ]; then
        echo "WARNING: Using potentially compatible headers package: $COMPATIBLE_HEADERS"
        echo "This may cause issues with NVIDIA driver compilation"
        install_packages_with_retry $COMPATIBLE_HEADERS
    else
        echo "FATAL: No compatible kernel headers found. Cannot proceed with NVIDIA driver installation."
        exit 1
    fi
else
    install_packages_with_retry linux-headers-$(uname -r)
fi

install_packages_with_retry apt-utils build-essential ca-certificates curl kmod file libelf-dev libglvnd-dev pkg-config make

install_packages_with_retry gcc-12 g++-12 && \
    update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12 && \
    update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

# Install the new cuda-keyring package	
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb
with_retry 3 10s sudo apt-get update

# Install the NVIDIA driver
install_packages_with_retry cuda-drivers-550

# Check if NVIDIA module is loaded, if not load it
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia
fi

# Start nvidia-persistenced
sudo nvidia-persistenced --persistence-mode

# Quick check to see if the driver is installed
nvidia-smi
`,
		},
		{
			description: "Version is preferred",
			driver: &NvDriver{
				Branch:  "550",
				Version: "123.4.5",
			},
			expectedOutput: `
# Install Dependencies
with_retry 3 10s sudo apt-get update

# Check if kernel headers are available for the current kernel
KERNEL_VERSION=$(uname -r)
echo "Checking for kernel headers for ${KERNEL_VERSION}..."

if ! apt-cache show linux-headers-${KERNEL_VERSION} >/dev/null 2>&1; then
    echo "ERROR: Kernel headers for ${KERNEL_VERSION} not found in repositories"
    echo "Available kernel headers:"
    apt-cache search linux-headers | grep -E "linux-headers-[0-9]" | sort
    
    # Try to find a compatible kernel header package
    KERNEL_BASE=$(echo ${KERNEL_VERSION} | cut -d- -f1-2)
    echo "Searching for compatible headers with base version ${KERNEL_BASE}..."
    COMPATIBLE_HEADERS=$(apt-cache search linux-headers | grep -E "linux-headers-${KERNEL_BASE}" | head -1 | awk '{print $1}')
    
    if [ -n "$COMPATIBLE_HEADERS" ]; then
        echo "WARNING: Using potentially compatible headers package: $COMPATIBLE_HEADERS"
        echo "This may cause issues with NVIDIA driver compilation"
        install_packages_with_retry $COMPATIBLE_HEADERS
    else
        echo "FATAL: No compatible kernel headers found. Cannot proceed with NVIDIA driver installation."
        exit 1
    fi
else
    install_packages_with_retry linux-headers-$(uname -r)
fi

install_packages_with_retry apt-utils build-essential ca-certificates curl kmod file libelf-dev libglvnd-dev pkg-config make

install_packages_with_retry gcc-12 g++-12 && \
    update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-12 12 && \
    update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-12 12

# Install the new cuda-keyring package	
distribution=$(. /etc/os-release;echo $ID$VERSION_ID | sed -e 's/\.//g')
wget https://developer.download.nvidia.com/compute/cuda/repos/$distribution/x86_64/cuda-keyring_1.1-1_all.deb
sudo dpkg -i cuda-keyring_1.1-1_all.deb
with_retry 3 10s sudo apt-get update

# Install the NVIDIA driver
install_packages_with_retry cuda-drivers=123.4.5

# Check if NVIDIA module is loaded, if not load it
if ! lsmod | grep -q "^nvidia "; then
    sudo modprobe nvidia
fi

# Start nvidia-persistenced
sudo nvidia-persistenced --persistence-mode

# Quick check to see if the driver is installed
nvidia-smi
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {

			var output bytes.Buffer

			err := tc.driver.Execute(&output, v1alpha1.Environment{})
			require.EqualValues(t, tc.expecteError, err)

			// Compare trimmed strings to avoid whitespace issues
			require.EqualValues(t, strings.TrimSpace(tc.expectedOutput), strings.TrimSpace(output.String()))
		})

	}
}
