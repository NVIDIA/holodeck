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
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/NVIDIA/holodeck/internal/logger"
)

// LoadConfig loads the AWS SDK configuration for region.
func LoadConfig(ctx context.Context, _ *logger.FunLogger, region string) (aws.Config, error) {
	configFile := os.Getenv("AWS_CONFIG_FILE")
	if configFile == "" {
		configFile = config.DefaultSharedConfigFilename()
	}
	credsFile := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credsFile == "" {
		credsFile = config.DefaultSharedCredentialsFilename()
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion(region),
		config.WithSharedConfigFiles([]string{configFile}),
		config.WithSharedCredentialsFiles([]string{credsFile}))
}
