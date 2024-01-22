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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (a *Client) checkInstanceTypes() error {
	var nextToken *string

	for {
		// Use the DescribeInstanceTypes API to get a list of supported instance types in the current region
		resp, err := a.ec2.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{NextToken: nextToken})
		if err != nil {
			return err
		}

		for _, it := range resp.InstanceTypes {
			if it.InstanceType == types.InstanceType(a.Spec.Instance.Type) {
				return nil
			}
		}

		if resp.NextToken != nil {
			nextToken = resp.NextToken
		} else {
			break
		}
	}

	return fmt.Errorf("instance type %s is not supported in the current region %s", string(a.Spec.Instance.Type), a.Spec.Instance.Region)
}

func (a *Client) checkImages() error {
	var nextToken *string

	for {
		// Use the DescribeImages API to get a list of supported images in the current region
		resp, err := a.ec2.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
			NextToken: nextToken,
		},
		)
		if err != nil {
			return err
		}

		for _, image := range resp.Images {
			if *image.ImageId == *a.Spec.Instance.Image.ImageId {
				return nil
			}
		}

		if resp.NextToken != nil {
			nextToken = resp.NextToken
		} else {
			break
		}
	}

	return fmt.Errorf("image %s is not supported in the current region %s", *a.Spec.Instance.Image.ImageId, a.Spec.Instance.Region)
}

func (a *Client) DryRun() error {
	// Check if the desired instance type is supported in the region
	fmt.Printf("Checking if instance type %s is supported in region %s\n", string(a.Spec.Instance.Type), a.Spec.Instance.Region)
	err := a.checkInstanceTypes()
	if err != nil {
		return err
	}

	// Check if the desired image is supported in the region
	fmt.Printf("Checking if image %s is supported in region %s\n", *a.Spec.Instance.Image.ImageId, a.Spec.Instance.Region)
	err = a.checkImages()
	if err != nil {
		return fmt.Errorf("failed to get images: %v", err)
	}

	return nil
}
