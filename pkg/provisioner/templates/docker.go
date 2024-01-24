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

const dockerTemplate = `

# Based on https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository
: ${DOCKER_VERSION:={{.Version}}}

# Add Docker's official GPG key:
sudo apt-get update
install_packages_with_retry ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# if DOCKER_VERSION is latest, then install latest version, else install specific version
if [ "$DOCKER_VERSION" = "latest" ]; then
  install_packages_with_retry docker-ce docker-ce-cli containerd.io
else
  install_packages_with_retry docker-ce=$DOCKER_VERSION docker-ce-cli=$DOCKER_VERSION containerd.io
fi

# Create required directories
sudo mkdir -p /etc/systemd/system/docker.service.d

# Create daemon json config file
sudo tee /etc/docker/daemon.json <<EOF
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m"
  },
  "storage-driver": "overlay2"
}
EOF

# Start and enable Services
sudo systemctl daemon-reload 
sudo systemctl enable docker
sudo systemctl restart docker

# Post-installation steps for Linux
sudo usermod -aG docker $USER
newgrp docker
`

type Docker struct {
	Version string
}

func NewDocker(env v1alpha1.Environment) *Docker {
	var version string

	if env.Spec.ContainerRuntime.Version != "" {
		version = env.Spec.ContainerRuntime.Version
	} else {
		version = "latest"
	}
	return &Docker{
		Version: version,
	}
}

func (t *Docker) Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error {
	dockerTemplate := template.Must(template.New("docker").Parse(dockerTemplate))
	if err := dockerTemplate.Execute(tpl, t); err != nil {
		return fmt.Errorf("failed to execute docker template: %v", err)
	}

	return nil
}
