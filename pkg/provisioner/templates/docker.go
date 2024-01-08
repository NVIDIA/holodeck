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

const Docker = `

# Based on https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository
: ${DOCKER_VERSION:={{.Version}}

# Add repo and Install packages
apt update
apt install -y curl gnupg software-properties-common apt-transport-https ca-certificates
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null
apt update

# if DOCKER_VERSION is latest, then install latest version, else install specific version
if [ "$DOCKER_VERSION" = "latest" ]; then
  apt install -y docker-ce docker-ce-cli containerd.io
else
  apt install -y docker-ce={{.DockerVersion}} docker-ce-cli={{.DockerVersion}} containerd.io
fi

# Create required directories
mkdir -p /etc/systemd/system/docker.service.d

# Create daemon json config file
tee /etc/docker/daemon.json <<EOF
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
systemctl daemon-reload 
systemctl enable docker
systemctl restart docker
`
