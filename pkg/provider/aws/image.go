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
	if p.Spec.Instance.Image.ImageId != nil {
		return p.assertImageIdSupported()
	}

	return p.setAMI()
}

func (p *Provider) setAMI() error {
	// If the image ID is already set by the user, return
	if p.Spec.Image.ImageId != nil {
		return nil
	}

	// Default to the official Ubuntu images in the AWS Marketplace
	// TODO: Add support for other image OS types
	awsOwner := []string{"099720109477", "679593333241"}
	if p.Spec.Instance.Image.OwnerId != nil {
		awsOwner = []string{*p.Spec.Instance.Image.OwnerId}
	}

	var filterNameValue []string
	var filterArchitectureValue []string

	if p.Spec.Instance.Image.Architecture != "" {
		switch p.Spec.Instance.Image.Architecture {
		case "x86_64", "amd64":
			filterArchitectureValue = []string{"x86_64", "amd64"}
		case "arm64", "aarch64":
			filterArchitectureValue = []string{"arm64"}
		default:
			return fmt.Errorf("invalid architecture %s", p.Spec.Instance.Image.Architecture)
		}
	}

	for _, arch := range filterArchitectureValue {
		filterNameValue = append(filterNameValue, fmt.Sprintf("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-20*", arch))
	}

	filter := []types.Filter{
		{
			Name:   aws.String("name"),
			Values: filterNameValue,
		},
		{
			Name:   aws.String("architecture"),
			Values: filterArchitectureValue,
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

	return nil
}

func (p *Provider) assertImageIdSupported() error {
	images, err := p.describeImages([]types.Filter{})
	if err == nil {
		for _, image := range images {
			if image.ImageID == *p.Spec.Instance.Image.ImageId {
				return nil
			}
		}
	}

	return errors.Join(err, fmt.Errorf("image %s is not supported in the current region %s", *p.Spec.Instance.Image.ImageId, p.Spec.Instance.Region))
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
