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
COMPONENT="kernel"
KERNEL_VERSION="{{ .Spec.Kernel.Version }}"

# Set non-interactive frontend for apt and disable editor prompts
export DEBIAN_FRONTEND=noninteractive
export EDITOR=/bin/true
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections

holodeck_progress "$COMPONENT" 1 4 "Checking current kernel"

# Ensure cloud-init's status is "done" before beginning any setup operations
/usr/bin/cloud-init status --wait || true

# Get current kernel version
CURRENT_KERNEL=$(uname -r)
holodeck_log "INFO" "$COMPONENT" "Current kernel: ${CURRENT_KERNEL}"
holodeck_log "INFO" "$COMPONENT" "Desired kernel: ${KERNEL_VERSION}"

# Check if already running the desired kernel
if [[ "${CURRENT_KERNEL}" == "${KERNEL_VERSION}" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Already running desired kernel ${KERNEL_VERSION}"
    holodeck_mark_installed "$COMPONENT" "$KERNEL_VERSION"
    exit 0
fi

# Check if state indicates we're waiting for reboot
STATE_FILE="${HOLODECK_STATE_DIR}/${COMPONENT}.state"
if [[ -f "$STATE_FILE" ]] && grep -q "status=pending_reboot" "$STATE_FILE"; then
    holodeck_log "WARN" "$COMPONENT" \
        "Kernel was installed but system wasn't rebooted properly"
    holodeck_log "INFO" "$COMPONENT" "Please reboot the system manually"
    exit 0
fi

holodeck_progress "$COMPONENT" 2 4 "Installing kernel packages"

# Update package lists
holodeck_retry 3 "$COMPONENT" sudo apt-get update

# Check if kernel packages are available
if ! apt-cache show "linux-image-${KERNEL_VERSION}" &>/dev/null; then
    holodeck_error 4 "$COMPONENT" \
        "Kernel version ${KERNEL_VERSION} not found in repositories" \
        "Check available kernels with: apt-cache search linux-image"
fi

holodeck_progress "$COMPONENT" 3 4 "Installing kernel ${KERNEL_VERSION}"

# Clean up old kernel files (optional, preserves space)
sudo rm -rf /boot/*"${CURRENT_KERNEL}"* 2>/dev/null || true
sudo rm -rf /lib/modules/*"${CURRENT_KERNEL}"* 2>/dev/null || true
sudo rm -rf /boot/*.old 2>/dev/null || true

# Install new kernel and related packages
holodeck_retry 3 "$COMPONENT" sudo apt-get install --allow-downgrades -y \
    "linux-image-${KERNEL_VERSION}" \
    "linux-headers-${KERNEL_VERSION}" \
    "linux-modules-${KERNEL_VERSION}"

holodeck_progress "$COMPONENT" 4 4 "Updating bootloader"

holodeck_log "INFO" "$COMPONENT" "Updating grub and initramfs"
sudo update-grub || true
sudo update-initramfs -u -k "${KERNEL_VERSION}" || true

# Mark as pending reboot
sudo tee "$STATE_FILE" > /dev/null <<EOF
status=pending_reboot
version=${KERNEL_VERSION}
installed_at=$(date -Iseconds)
EOF

holodeck_log "INFO" "$COMPONENT" "Kernel ${KERNEL_VERSION} installed, rebooting..."

# Run the reboot command with nohup to avoid abrupt SSH closure issues
nohup sudo reboot &
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
