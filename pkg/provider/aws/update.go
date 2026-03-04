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

	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Update updates an AWS resources tags
func (p *Provider) UpdateResourcesTags(tags map[string]string, resources ...string) error {
	cancel := p.log.Loading("Tagging AWS resources...")

	var awsTags []types.Tag
	for k, v := range tags {
		awsTags = append(awsTags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	createTagsIn := &ec2.CreateTagsInput{
		Resources: resources,
		Tags:      awsTags,
	}

	_, err := p.ec2.CreateTags(context.Background(), createTagsIn)
	if err != nil {
		cancel(logger.ErrLoadingFailed)
		return err
	}
	cancel(nil)

	return nil
}
