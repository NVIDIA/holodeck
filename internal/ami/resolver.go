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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// EC2ImageDescriber defines the subset of EC2 operations needed for AMI
// resolution.
type EC2ImageDescriber interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

// SSMParameterGetter defines the subset of SSM operations needed for AMI
// resolution.
type SSMParameterGetter interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Resolver resolves OS IDs to AMI IDs using SSM Parameter Store or
// EC2 DescribeImages API.
type Resolver struct {
	ec2Client EC2ImageDescriber
	ssmClient SSMParameterGetter
	region    string
}

// NewResolver creates a new AMI resolver.
func NewResolver(
	ec2Client EC2ImageDescriber,
	ssmClient SSMParameterGetter,
	region string,
) *Resolver {
	return &Resolver{
		ec2Client: ec2Client,
		ssmClient: ssmClient,
		region:    region,
	}
}

// Resolve looks up the AMI for the given OS ID, region, and architecture.
// It first tries SSM Parameter Store (if available for the OS), then falls
// back to EC2 DescribeImages API.
func (r *Resolver) Resolve(
	ctx context.Context,
	osID, arch string,
) (*ResolvedAMI, error) {
	// Normalize architecture
	arch = NormalizeArch(arch)

	// Get OS metadata
	osImage, ok := Get(osID)
	if !ok {
		return nil, fmt.Errorf(
			"unknown OS: %s (run 'holodeck os list' for available options)",
			osID,
		)
	}

	// Validate architecture
	if !contains(osImage.Architectures, arch) {
		return nil, fmt.Errorf(
			"OS %s does not support architecture %s (supported: %s)",
			osID, arch, strings.Join(osImage.Architectures, ", "),
		)
	}

	// Try SSM first if available (fastest, always up-to-date)
	if osImage.SSMPath != "" && r.ssmClient != nil {
		amiID, err := r.resolveViaSSM(ctx, osImage, arch)
		if err == nil {
			return &ResolvedAMI{
				ImageID:        amiID,
				SSHUsername:    osImage.SSHUsername,
				OSFamily:       osImage.Family,
				PackageManager: osImage.PackageManager,
			}, nil
		}
		// Fall through to DescribeImages on SSM failure
	}

	// Fall back to DescribeImages
	amiID, err := r.resolveViaDescribeImages(ctx, osImage, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve AMI for %s: %w", osID, err)
	}

	return &ResolvedAMI{
		ImageID:        amiID,
		SSHUsername:    osImage.SSHUsername,
		OSFamily:       osImage.Family,
		PackageManager: osImage.PackageManager,
	}, nil
}

// resolveViaSSM retrieves the AMI ID from AWS SSM Parameter Store.
func (r *Resolver) resolveViaSSM(
	ctx context.Context,
	osImage *OSImage,
	arch string,
) (string, error) {
	ssmArch := archToSSMArch(arch)
	paramName := fmt.Sprintf(osImage.SSMPath, ssmArch)

	result, err := r.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(paramName),
	})
	if err != nil {
		return "", fmt.Errorf("SSM lookup failed: %w", err)
	}

	if result.Parameter == nil || result.Parameter.Value == nil {
		return "", fmt.Errorf("SSM parameter %s has no value", paramName)
	}

	return *result.Parameter.Value, nil
}

// resolveViaDescribeImages retrieves the AMI ID using EC2 DescribeImages API.
func (r *Resolver) resolveViaDescribeImages(
	ctx context.Context,
	osImage *OSImage,
	arch string,
) (string, error) {
	// Convert architecture for AMI name pattern (Ubuntu uses "amd64" not "x86_64")
	nameArch := archToAMINameArch(arch)
	namePattern := fmt.Sprintf(osImage.NamePattern, nameArch)

	filters := []types.Filter{
		{
			Name:   aws.String("name"),
			Values: []string{namePattern},
		},
		{
			Name:   aws.String("architecture"),
			Values: []string{arch},
		},
		{
			Name:   aws.String("state"),
			Values: []string{"available"},
		},
	}

	input := &ec2.DescribeImagesInput{
		Filters: filters,
	}

	// Handle owner specification
	if osImage.OwnerID == "amazon" {
		input.Owners = []string{"amazon"}
	} else {
		filters = append(filters, types.Filter{
			Name:   aws.String("owner-id"),
			Values: []string{osImage.OwnerID},
		})
		input.Filters = filters
	}

	result, err := r.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return "", fmt.Errorf("DescribeImages failed: %w", err)
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf(
			"no images found for %s in region %s",
			osImage.ID, r.region,
		)
	}

	// Sort by creation date (newest first)
	sort.Slice(result.Images, func(i, j int) bool {
		return aws.ToString(result.Images[i].CreationDate) >
			aws.ToString(result.Images[j].CreationDate)
	})

	imageID := aws.ToString(result.Images[0].ImageId)
	if imageID == "" {
		return "", fmt.Errorf("AMI has empty ImageId for %s", osImage.ID)
	}

	return imageID, nil
}

// NormalizeArch converts architecture aliases to canonical form.
func NormalizeArch(arch string) string {
	switch strings.ToLower(arch) {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return arch
	}
}

// archToSSMArch converts EC2 architecture names to SSM parameter path format.
// SSM uses "amd64" while EC2 uses "x86_64".
func archToSSMArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	default:
		return arch
	}
}

// archToAMINameArch converts EC2 architecture names to AMI name pattern format.
// Some vendors (like Ubuntu) use "amd64" in AMI names instead of "x86_64".
func archToAMINameArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	default:
		return arch
	}
}

// contains checks if a string slice contains a specific item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
