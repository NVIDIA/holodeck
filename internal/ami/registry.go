/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package ami

import (
	"sort"
)

// registry contains the mapping of OS identifiers to their metadata.
var registry = map[string]OSImage{
	"ubuntu-24.04": {
		ID:              "ubuntu-24.04",
		Name:            "Ubuntu 24.04 LTS (Noble Numbat)",
		Family:          OSFamilyDebian,
		SSHUsername:     "ubuntu",
		PackageManager:  PackageManagerAPT,
		MinRootVolumeGB: 20,
		OwnerID:         "099720109477",
		NamePattern:     "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-%s-server-*",
		SSMPath: "/aws/service/canonical/ubuntu/server/24.04/stable/" +
			"current/%s/hvm/ebs-gp3/ami-id",
		Architectures: []string{"x86_64", "arm64"},
	},
	"ubuntu-22.04": {
		ID:              "ubuntu-22.04",
		Name:            "Ubuntu 22.04 LTS (Jammy Jellyfish)",
		Family:          OSFamilyDebian,
		SSHUsername:     "ubuntu",
		PackageManager:  PackageManagerAPT,
		MinRootVolumeGB: 20,
		OwnerID:         "099720109477",
		NamePattern:     "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-*",
		SSMPath: "/aws/service/canonical/ubuntu/server/22.04/stable/" +
			"current/%s/hvm/ebs-gp3/ami-id",
		Architectures: []string{"x86_64", "arm64"},
	},
	"amazon-linux-2023": {
		ID:              "amazon-linux-2023",
		Name:            "Amazon Linux 2023",
		Family:          OSFamilyAmazon,
		SSHUsername:     "ec2-user",
		PackageManager:  PackageManagerDNF,
		MinRootVolumeGB: 20,
		OwnerID:         "amazon",
		NamePattern:     "al2023-ami-*-kernel-*-%s",
		SSMPath: "/aws/service/ami-amazon-linux-latest/" +
			"al2023-ami-kernel-default-%s",
		Architectures: []string{"x86_64", "arm64"},
	},
	"rocky-9": {
		ID:              "rocky-9",
		Name:            "Rocky Linux 9",
		Family:          OSFamilyRHEL,
		SSHUsername:     "rocky",
		PackageManager:  PackageManagerDNF,
		MinRootVolumeGB: 20,
		OwnerID:         "792107900819",
		NamePattern:     "Rocky-9-EC2-Base-*.%s-*",
		SSMPath:         "", // No SSM support for Rocky Linux
		Architectures:   []string{"x86_64", "arm64"},
	},
}

// Get returns an OSImage by ID. Returns nil and false if not found.
func Get(id string) (*OSImage, bool) {
	img, ok := registry[id]
	if !ok {
		return nil, false
	}
	return &img, true
}

// List returns all supported OS IDs in sorted order.
func List() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// All returns all OSImage entries in sorted order by ID.
func All() []OSImage {
	images := make([]OSImage, 0, len(registry))
	for _, id := range List() {
		images = append(images, registry[id])
	}
	return images
}

// Exists checks if an OS ID is in the registry.
func Exists(id string) bool {
	_, ok := registry[id]
	return ok
}
