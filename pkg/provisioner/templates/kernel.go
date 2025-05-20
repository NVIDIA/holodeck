/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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
{{- if .Spec.Kernel.Version }}
# Set non-interactive frontend for apt and disable editor prompts
export DEBIAN_FRONTEND=noninteractive
export EDITOR=/bin/true
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections

# Get current kernel version
CURRENT_KERNEL=$(uname -r)
echo "Current kernel version: $CURRENT_KERNEL"

KERNEL_VERSION="{{ .Spec.Kernel.Version }}"

if [ "${CURRENT_KERNEL}" != "${KERNEL_VERSION}" ]; then
    echo "--------------Upgrading kernel to ${KERNEL_VERSION}--------------"
    
    # Update package lists
    sudo apt-get update -y || true
    
    # Clean up old kernel files
    sudo rm -rf /boot/*${CURRENT_KERNEL}* || true
    sudo rm -rf /lib/modules/*${CURRENT_KERNEL}*
    sudo rm -rf /boot/*.old
    
    # Install new kernel and related packages
    sudo apt-get install --allow-downgrades \
        linux-image-${KERNEL_VERSION} \
        linux-headers-${KERNEL_VERSION} \
        linux-modules-${KERNEL_VERSION} -y || exit 1
     
    echo "Updating grub and initramfs..."
    sudo update-grub || true
    sudo update-initramfs -u -k ${KERNEL_VERSION} || true
    
    echo "Rebooting..."
    # Run the reboot command with nohup to avoid abrupt SSH closure issues
    nohup sudo reboot &
    
    echo "--------------Kernel upgrade completed--------------"
else
    echo "--------------Kernel upgrade not required, current kernel version ${KERNEL_VERSION}--------------"
fi
{{- end }}
`

// NewKernelTemplate creates a new kernel template
func NewKernelTemplate(env v1alpha1.Environment) (*bytes.Buffer, error) {
	tmpl, err := template.New("kernel").Parse(kernelTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, env); err != nil {
		return nil, fmt.Errorf("failed to execute kernel template: %w", err)
	}

	return &buf, nil
}
