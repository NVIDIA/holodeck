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

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMClient defines the interface for SSM operations used throughout holodeck.
// This interface enables dependency injection and facilitates unit testing
// by allowing mock implementations to be substituted for the real SSM client.
type SSMClient interface {
	// GetParameter retrieves a parameter value from SSM Parameter Store.
	GetParameter(ctx context.Context, params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Ensure *ssm.Client implements SSMClient at compile time.
var _ SSMClient = (*ssm.Client)(nil)
