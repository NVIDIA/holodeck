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

// Package ami provides internal AMI-to-OS mapping utilities for holodeck.
// This package enables simplified configuration by allowing users to specify
// an OS identifier instead of a specific AMI ID.
package ami

// OSFamily groups similar operating systems for template selection and
// package management.
type OSFamily string

const (
	// OSFamilyDebian includes Ubuntu, Debian, and similar distributions.
	OSFamilyDebian OSFamily = "debian"
	// OSFamilyRHEL includes Rocky Linux, Fedora, RHEL, CentOS, and similar.
	OSFamilyRHEL OSFamily = "rhel"
	// OSFamilyAmazon includes Amazon Linux distributions.
	OSFamilyAmazon OSFamily = "amazon"
)

// PackageManager indicates the package management system used by an OS.
type PackageManager string

const (
	// PackageManagerAPT is the Advanced Package Tool used by Debian-based
	// distributions.
	PackageManagerAPT PackageManager = "apt"
	// PackageManagerDNF is the Dandified YUM package manager used by modern
	// RHEL-based distributions.
	PackageManagerDNF PackageManager = "dnf"
	// PackageManagerYUM is the Yellowdog Updater Modified package manager
	// used by older RHEL-based distributions.
	PackageManagerYUM PackageManager = "yum"
)

// OSImage defines metadata for an operating system image.
type OSImage struct {
	// ID is the short identifier (e.g., "ubuntu-22.04").
	ID string

	// Name is the display name (e.g., "Ubuntu 22.04 LTS (Jammy Jellyfish)").
	Name string

	// Family groups related OSes for template selection.
	Family OSFamily

	// SSHUsername is the default SSH user for this OS.
	SSHUsername string

	// PackageManager indicates the package management system.
	PackageManager PackageManager

	// MinRootVolumeGB is the minimum root volume size in gigabytes.
	MinRootVolumeGB int32

	// OwnerID is the AWS account ID that owns the AMI, used as a filter in
	// DescribeImages. Use "amazon" for Amazon-owned images.
	OwnerID string

	// NamePattern is the pattern for DescribeImages filter. Use %s as a
	// placeholder for architecture (e.g., "x86_64" or "arm64").
	NamePattern string

	// SSMPath is the SSM Parameter Store path for looking up the latest AMI.
	// Use %s as a placeholder for architecture. Empty if SSM is not supported.
	SSMPath string

	// Architectures lists the supported CPU architectures (x86_64, arm64).
	Architectures []string
}

// ResolvedAMI contains the resolved AMI information along with metadata
// needed for provisioning.
type ResolvedAMI struct {
	// ImageID is the resolved AMI ID.
	ImageID string

	// SSHUsername is the default SSH user for this OS.
	SSHUsername string

	// OSFamily is the family of the resolved OS.
	OSFamily OSFamily

	// PackageManager is the package manager used by the resolved OS.
	PackageManager PackageManager
}
