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
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add the repository to Apt sources:
echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  with_retry 3 10s sudo apt-get update

# Install containerd
install_packages_with_retry containerd.io=${CONTAINERD_VERSION}-1

# Configure containerd and start service
mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml
# Set systemd as the cgroup driver 
# see https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd
sudo sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

# restart containerd
sudo systemctl restart containerd
sudo systemctl enable containerd
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
