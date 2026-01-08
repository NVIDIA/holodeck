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

const containerToolkitTemplate = `
COMPONENT="nvidia-container-toolkit"
CONTAINER_RUNTIME="{{.ContainerRuntime}}"
ENABLE_CDI="{{.EnableCDI}}"

holodeck_progress "$COMPONENT" 1 4 "Checking existing installation"

# Check if NVIDIA Container Toolkit is already installed and functional
TOOLKIT_NEEDS_CONFIG=true
if command -v nvidia-ctk &>/dev/null; then
    INSTALLED_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || true)
    if [[ -n "$INSTALLED_VERSION" ]]; then
        holodeck_log "INFO" "$COMPONENT" "Already installed: ${INSTALLED_VERSION}"

        if holodeck_verify_toolkit; then
            holodeck_log "INFO" "$COMPONENT" "Toolkit verified functional"

            # Check if runtime configuration is already correct
            if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
                if sudo grep -q "nvidia-container-runtime" /etc/containerd/config.toml 2>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" \
                        "Runtime ${CONTAINER_RUNTIME} already configured for NVIDIA"
                    holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                    exit 0
                fi
            elif [[ "${CONTAINER_RUNTIME}" == "docker" ]]; then
                if sudo grep -q "nvidia-container-runtime" /etc/docker/daemon.json 2>/dev/null; then
                    holodeck_log "INFO" "$COMPONENT" \
                        "Runtime ${CONTAINER_RUNTIME} already configured for NVIDIA"
                    holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                    exit 0
                fi
            elif [[ "${CONTAINER_RUNTIME}" == "crio" ]]; then
                if [[ -f /etc/crio/crio.conf.d/nvidia.conf ]]; then
                    holodeck_log "INFO" "$COMPONENT" \
                        "Runtime ${CONTAINER_RUNTIME} already configured for NVIDIA"
                    holodeck_mark_installed "$COMPONENT" "$INSTALLED_VERSION"
                    exit 0
                fi
            fi
            holodeck_log "INFO" "$COMPONENT" \
                "Toolkit installed but runtime configuration needed"
            TOOLKIT_NEEDS_CONFIG=true
        else
            holodeck_log "WARN" "$COMPONENT" \
                "Toolkit installed but not functional, attempting repair"
        fi
    fi
fi

holodeck_progress "$COMPONENT" 2 4 "Adding NVIDIA Container Toolkit repository"

# Add repository (idempotent)
if [[ ! -f /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg ]]; then
    holodeck_retry 3 "$COMPONENT" curl -fsSL \
        https://nvidia.github.io/libnvidia-container/gpgkey | \
        sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
else
    holodeck_log "INFO" "$COMPONENT" "NVIDIA Container Toolkit GPG key already present"
fi

if [[ ! -f /etc/apt/sources.list.d/nvidia-container-toolkit.list ]]; then
    curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
        sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
        sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list > /dev/null
else
    holodeck_log "INFO" "$COMPONENT" "NVIDIA Container Toolkit repository already configured"
fi

holodeck_retry 3 "$COMPONENT" sudo apt-get update

holodeck_progress "$COMPONENT" 3 4 "Installing NVIDIA Container Toolkit"

holodeck_retry 3 "$COMPONENT" install_packages_with_retry \
    nvidia-container-toolkit nvidia-container-toolkit-base \
    libnvidia-container-tools libnvidia-container1

holodeck_progress "$COMPONENT" 4 4 "Configuring runtime"

# Configure container runtime
holodeck_log "INFO" "$COMPONENT" \
    "Configuring ${CONTAINER_RUNTIME} with CDI=${ENABLE_CDI}"
sudo nvidia-ctk runtime configure \
    --runtime="${CONTAINER_RUNTIME}" \
    --set-as-default \
    --enable-cdi="${ENABLE_CDI}"

# Verify CNI configuration is preserved after nvidia-ctk
if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    holodeck_log "INFO" "$COMPONENT" "Verifying CNI configuration after nvidia-ctk"
    if ! sudo grep -q 'bin_dir = "/opt/cni/bin"' /etc/containerd/config.toml 2>/dev/null; then
        holodeck_log "WARN" "$COMPONENT" \
            "CNI bin_dir configuration may have been modified by nvidia-ctk"
        holodeck_log "INFO" "$COMPONENT" "Restoring CNI paths"
        sudo sed -i '/\[plugins."io.containerd.grpc.v1.cri".cni\]/,/\[/{s|bin_dir = .*|bin_dir = "/opt/cni/bin"|g}' \
            /etc/containerd/config.toml
    fi
fi

sudo systemctl restart "${CONTAINER_RUNTIME}"

# Verify toolkit is functional
if ! holodeck_verify_toolkit; then
    holodeck_error 12 "$COMPONENT" \
        "NVIDIA Container Toolkit verification failed" \
        "Run 'nvidia-ctk --version' and check ${CONTAINER_RUNTIME} logs"
fi

FINAL_VERSION=$(nvidia-ctk --version 2>/dev/null | head -1 || echo "installed")
holodeck_mark_installed "$COMPONENT" "$FINAL_VERSION"
holodeck_log "INFO" "$COMPONENT" \
    "Successfully installed NVIDIA Container Toolkit for ${CONTAINER_RUNTIME}"
`

type ContainerToolkit struct {
	ContainerRuntime string
	EnableCDI        bool
}

func NewContainerToolkit(env v1alpha1.Environment) *ContainerToolkit {
	runtime := string(env.Spec.ContainerRuntime.Name)
	if runtime == "" {
		runtime = "containerd"
	}

	ctk := &ContainerToolkit{
		ContainerRuntime: runtime,
		EnableCDI:        env.Spec.NVIDIAContainerToolkit.EnableCDI,
	}

	return ctk
}

func (t *ContainerToolkit) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	containerTlktTemplate := template.Must(template.New("container-toolkit").Parse(containerToolkitTemplate))
	if err := containerTlktTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute container-toolkit template: %v", err)
	}

	return nil
}
