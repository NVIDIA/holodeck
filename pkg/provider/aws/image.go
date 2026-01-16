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

	// Auto-set username if not provided
	//nolint:staticcheck // Auth is embedded but explicit access is clearer
	if p.Spec.Auth.Username == "" {
		//nolint:staticcheck // Auth is embedded but explicit access is clearer
		p.Spec.Auth.Username = resolved.SSHUsername
	}

	return nil
}

// setLegacyAMI implements the original Ubuntu 22.04 default behavior for
// backward compatibility when OS is not specified.
func (p *Provider) setLegacyAMI() error {
	// Default to the official Ubuntu images in the AWS Marketplace
	awsOwner := []string{"099720109477", "679593333241"}
	if p.Spec.Image.OwnerId != nil {
		awsOwner = []string{*p.Spec.Image.OwnerId}
	}

	// Validate and normalize architecture
	var arch string
	switch p.Spec.Image.Architecture {
	case "x86_64", "amd64":
		arch = "x86_64"
	case "arm64", "aarch64":
		arch = "arm64"
	case "":
		arch = "x86_64" // Default
	default:
		return fmt.Errorf("invalid architecture %s", p.Spec.Image.Architecture)
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
		return fmt.Errorf("failed to describe images: %w", err)
	}

	if len(images) == 0 {
		return fmt.Errorf("no images found")
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].CreationDate > images[j].CreationDate
	})
	p.Spec.Image.ImageId = &images[0].ImageID

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
