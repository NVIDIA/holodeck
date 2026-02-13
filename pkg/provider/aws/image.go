/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type ImageInfo struct {
	ImageID      string
	CreationDate string
}

type ByCreationDate []ImageInfo

func (a ByCreationDate) Len() int           { return len(a) }
func (a ByCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCreationDate) Less(i, j int) bool { return a[i].CreationDate < a[j].CreationDate }

func (p *Provider) checkImages() error {
	// Check if the given image is supported in the region
	if p.Spec.Image.ImageId != nil {
		return p.assertImageIdSupported()
	}

	return p.setAMI()
}

func (p *Provider) setAMI() error {
	// If the image ID is already set by the user, return
	if p.Spec.Image.ImageId != nil {
		return nil
	}

	// If OS is specified, use the AMI resolver
	//nolint:staticcheck // Instance is embedded but explicit access is clearer
	if p.Spec.Instance.OS != "" {
		return p.resolveOSToAMI()
	}

	// Fall back to legacy behavior: Ubuntu 22.04 by default
	return p.setLegacyAMI()
}

// resolveOSToAMI uses the AMI resolver to look up the AMI for the specified OS.
func (p *Provider) resolveOSToAMI() error {
	arch := p.Spec.Image.Architecture
	if arch == "" {
		arch = "x86_64" // Default architecture
	}

	//nolint:staticcheck // Instance is embedded but explicit access is clearer
	resolved, err := p.amiResolver.Resolve(
		context.TODO(),
		p.Spec.Instance.OS,
		arch,
	)
	if err != nil {
		//nolint:staticcheck // Instance is embedded but explicit access is clearer
		return fmt.Errorf(
			"failed to resolve AMI for OS %s: %w",
			p.Spec.Instance.OS,
			err,
		)
	}

	p.Spec.Image.ImageId = &resolved.ImageID
	p.Spec.Image.Architecture = normalizeArchToEC2(arch)

	// Auto-set username if not provided
	//nolint:staticcheck // Auth is embedded but explicit access is clearer
	if p.Spec.Auth.Username == "" {
		//nolint:staticcheck // Auth is embedded but explicit access is clearer
		p.Spec.Auth.Username = resolved.SSHUsername
	}

	return nil
}

// ResolvedImage contains the resolved AMI information for instance creation.
type ResolvedImage struct {
	ImageID      string
	SSHUsername  string
	Architecture string // EC2 architecture: "x86_64" or "arm64"
}

// resolveImageForNode resolves the AMI for a node based on OS or explicit Image.
// This method does not mutate provider state, making it safe for cluster mode
// where different node pools may use different images.
func (p *Provider) resolveImageForNode(os string, image *v1alpha1.Image, arch string) (*ResolvedImage, error) {
	// If explicit ImageId is provided, use it
	if image != nil && image.ImageId != nil && *image.ImageId != "" {
		arch, err := p.describeImageArch(*image.ImageId)
		if err != nil {
			return nil, fmt.Errorf("failed to determine architecture for image %s: %w", *image.ImageId, err)
		}
		return &ResolvedImage{
			ImageID:      *image.ImageId,
			SSHUsername:  "", // Username must be provided in auth config
			Architecture: arch,
		}, nil
	}

	// Determine architecture
	if arch == "" {
		if image != nil && image.Architecture != "" {
			arch = image.Architecture
		} else {
			arch = "x86_64" // Default
		}
	}
	arch = normalizeArchToEC2(arch)

	// If OS is specified, resolve via AMI resolver
	if os != "" {
		resolved, err := p.amiResolver.Resolve(context.TODO(), os, arch)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve AMI for OS %s: %w", os, err)
		}
		return &ResolvedImage{
			ImageID:      resolved.ImageID,
			SSHUsername:  resolved.SSHUsername,
			Architecture: arch,
		}, nil
	}

	// Fall back to legacy behavior: use Instance.OS or default Ubuntu 22.04
	//nolint:staticcheck // Instance is embedded but explicit access is clearer
	if p.Spec.Instance.OS != "" {
		resolved, err := p.amiResolver.Resolve(context.TODO(), p.Spec.Instance.OS, arch)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve AMI for OS %s: %w", p.Spec.Instance.OS, err)
		}
		return &ResolvedImage{
			ImageID:      resolved.ImageID,
			SSHUsername:  resolved.SSHUsername,
			Architecture: arch,
		}, nil
	}

	// Fall back to legacy AMI lookup for Ubuntu 22.04
	// Use findLegacyAMI to avoid mutating provider state
	imageID, err := p.findLegacyAMI(arch)
	if err != nil {
		return nil, err
	}
	return &ResolvedImage{
		ImageID:      imageID,
		SSHUsername:  "ubuntu",
		Architecture: arch,
	}, nil
}

// findLegacyAMI looks up the latest Ubuntu 22.04 AMI without mutating state.
// This is a pure query function safe for use in cluster mode.
func (p *Provider) findLegacyAMI(arch string) (string, error) {
	// Default to the official Ubuntu images in the AWS Marketplace
	awsOwner := []string{"099720109477", "679593333241"}
	if p.Spec.Image.OwnerId != nil {
		awsOwner = []string{*p.Spec.Image.OwnerId}
	}

	// Validate and normalize architecture (case-insensitive for backward compatibility)
	if arch == "" {
		arch = p.Spec.Image.Architecture
	}
	switch strings.ToLower(arch) {
	case "x86_64", "amd64":
		arch = "x86_64"
	case "arm64", "aarch64":
		arch = "arm64"
	case "":
		arch = "x86_64" // Default
	default:
		return "", fmt.Errorf("invalid architecture %s", arch)
	}

	// Ubuntu AMI names use "amd64" not "x86_64"
	nameArch := arch
	if arch == "x86_64" {
		nameArch = "amd64"
	}
	filterNameValue := []string{
		fmt.Sprintf(
			"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-20*",
			nameArch,
		),
	}

	filter := []types.Filter{
		{
			Name:   aws.String("name"),
			Values: filterNameValue,
		},
		{
			Name:   aws.String("architecture"),
			Values: []string{arch},
		},
		{
			Name:   aws.String("owner-id"),
			Values: awsOwner,
		},
	}

	images, err := p.describeImages(filter)
	if err != nil {
		return "", fmt.Errorf("failed to describe images: %w", err)
	}

	if len(images) == 0 {
		return "", fmt.Errorf("no images found for Ubuntu 22.04 (%s)", arch)
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].CreationDate > images[j].CreationDate
	})

	return images[0].ImageID, nil
}

// setLegacyAMI implements the original Ubuntu 22.04 default behavior for
// backward compatibility when OS is not specified. This mutates provider state.
func (p *Provider) setLegacyAMI() error {
	imageID, err := p.findLegacyAMI("")
	if err != nil {
		return err
	}
	p.Spec.Image.ImageId = &imageID

	// Store the resolved architecture (normalized to EC2 form) for cross-validation in DryRun
	if p.Spec.Image.Architecture == "" {
		p.Spec.Image.Architecture = "x86_64" // Legacy default
	} else {
		p.Spec.Image.Architecture = normalizeArchToEC2(p.Spec.Image.Architecture)
	}

	// Set default username for Ubuntu if not provided
	//nolint:staticcheck // Auth is embedded but explicit access is clearer
	if p.Spec.Auth.Username == "" {
		p.Spec.Auth.Username = "ubuntu"
	}

	return nil
}

func (p *Provider) assertImageIdSupported() error {
	images, err := p.describeImages([]types.Filter{})
	if err == nil {
		for _, image := range images {
			if image.ImageID == *p.Spec.Image.ImageId {
				return nil
			}
		}
	}

	return errors.Join(err, fmt.Errorf("image %s is not supported in the current region %s", *p.Spec.Image.ImageId, p.Spec.Region))
}

func (p *Provider) describeImages(filter []types.Filter) ([]ImageInfo, error) {
	var images []ImageInfo
	var nextToken *string

	for {
		// Use the DescribeImages API to get a list of supported images in the current region
		resp, err := p.ec2.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
			NextToken: nextToken,
			Filters:   filter,
		})
		if err != nil {
			return images, fmt.Errorf("failed to describe images: %w", err)
		}
		if len(resp.Images) == 0 {
			return images, fmt.Errorf("no images found")
		}

		for _, image := range resp.Images {
			images = append(images, ImageInfo{
				ImageID:      *image.ImageId,
				CreationDate: *image.CreationDate,
			})
		}

		if resp.NextToken != nil {
			nextToken = resp.NextToken
		} else {
			break
		}
	}

	return images, nil
}

func (p *Provider) checkInstanceTypes() error {
	var nextToken *string

	for {
		// Use the DescribeInstanceTypes API to get a list of supported instance types in the current region
		resp, err := p.ec2.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{NextToken: nextToken})
		if err != nil {
			return err
		}

		for _, it := range resp.InstanceTypes {
			if it.InstanceType == types.InstanceType(p.Spec.Type) {
				return nil
			}
		}

		if resp.NextToken != nil {
			nextToken = resp.NextToken
		} else {
			break
		}
	}

	return fmt.Errorf("instance type %s is not supported in the current region %s", p.Spec.Type, p.Spec.Region)
}

// normalizeArchToEC2 converts architecture aliases to EC2 canonical form.
// EC2 APIs use "x86_64" and "arm64", but users and other systems may use
// "amd64" (Debian convention) or "aarch64" (kernel convention).
func normalizeArchToEC2(arch string) string {
	switch strings.ToLower(arch) {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return arch
	}
}

// describeImageArch queries EC2 DescribeImages for a specific AMI ID and
// returns its architecture string (e.g., "x86_64" or "arm64").
func (p *Provider) describeImageArch(imageID string) (string, error) {
	resp, err := p.ec2.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
		ImageIds: []string{imageID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe image %s: %w", imageID, err)
	}
	if len(resp.Images) == 0 {
		return "", fmt.Errorf("image %s not found", imageID)
	}
	return string(resp.Images[0].Architecture), nil
}

// getInstanceTypeArch queries EC2 DescribeInstanceTypes for a specific instance
// type and returns its list of supported architecture strings.
func (p *Provider) getInstanceTypeArch(instanceType string) ([]string, error) {
	resp, err := p.ec2.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{types.InstanceType(instanceType)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance type %s: %w", instanceType, err)
	}
	if len(resp.InstanceTypes) == 0 {
		return nil, fmt.Errorf("instance type %s not found", instanceType)
	}
	if resp.InstanceTypes[0].ProcessorInfo == nil {
		return nil, fmt.Errorf("no processor info for instance type %s", instanceType)
	}
	var archs []string
	for _, a := range resp.InstanceTypes[0].ProcessorInfo.SupportedArchitectures {
		archs = append(archs, string(a))
	}
	return archs, nil
}
