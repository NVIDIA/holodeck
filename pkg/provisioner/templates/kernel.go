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

const kernelTemplate = `
{{- if .Spec.Kernel.Modify }}
# Install required packages for kernel compilation
sudo apt-get update
install_packages_with_retry build-essential libncurses-dev bison flex libssl-dev libelf-dev

# Download and extract kernel source
cd /usr/src
{{- if .Spec.Kernel.Version }}
KERNEL_VERSION="{{ .Spec.Kernel.Version }}"
{{- else }}
# Get latest stable kernel version
KERNEL_VERSION=$(curl -s https://www.kernel.org/releases.json | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
{{- end }}

# Download and extract the kernel source
with_retry 3 5 wget https://cdn.kernel.org/pub/linux/kernel/v${KERNEL_VERSION%%.*}.x/linux-${KERNEL_VERSION}.tar.xz
tar xf linux-${KERNEL_VERSION}.tar.xz
cd linux-${KERNEL_VERSION}

# Configure and compile the kernel
make defconfig
with_retry 3 5 make -j$(nproc)

# Install the new kernel
sudo make modules_install
sudo make install

# Update GRUB to use the new kernel
sudo update-grub

# Clean up build files
cd /usr/src
sudo rm -rf linux-${KERNEL_VERSION}*

# Reboot to apply the new kernel
echo "Rebooting to apply kernel changes..."
sudo reboot
{{- end }}
`

// NewKernelTemplate creates a new kernel template
func NewKernelTemplate(env v1alpha1.Environment) (*bytes.Buffer, error) {
	tmpl, err := template.New("kernel").Parse(kernelTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, env); err != nil {
		return nil, fmt.Errorf("failed to execute kernel template: %v", err)
	}

	return &buf, nil
}
