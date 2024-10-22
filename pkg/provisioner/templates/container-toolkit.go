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

# Install container toolkit
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
  && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list \
  && \
    sudo apt-get update

sudo apt-get install -y nvidia-container-toolkit

# Configure container runtime
sudo nvidia-ctk runtime configure --runtime={{.ContainerRuntime}} --set-as-default
sudo systemctl restart {{.ContainerRuntime}}
`

type ContainerToolkit struct {
	ContainerRuntime string
}

func NewContainerToolkit(env v1alpha1.Environment) *ContainerToolkit {
	runtime := string(env.Spec.ContainerRuntime.Name)
	if runtime == "" {
		runtime = "containerd"
	}
	return &ContainerToolkit{
		ContainerRuntime: runtime,
	}
}

func (t *ContainerToolkit) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	containerTlktTemplate := template.Must(template.New("container-toolkit").Parse(containerToolkitTemplate))
	if err := containerTlktTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute container-toolkit template: %v", err)
	}

	if err := containerTlktTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute container-toolkit template: %v", err)
	}

	return nil
}
