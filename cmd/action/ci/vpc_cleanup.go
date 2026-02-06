/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

package ci

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NVIDIA/holodeck/internal/logger"
	cleanuppkg "github.com/NVIDIA/holodeck/pkg/cleanup"
)

// RunCleanup performs standalone VPC cleanup based on INPUT_VPC_IDS.
// This mode is used for periodic cleanup workflows.
func RunCleanup(log *logger.FunLogger) error {
	log.Info("Running VPC Cleanup action")

	// Read AWS credentials from inputs
	if err := readInputs(); err != nil {
		return err
	}

	// Get VPC IDs from input
	vpcIDsStr := os.Getenv("INPUT_VPC_IDS")
	if vpcIDsStr == "" {
		return fmt.Errorf("INPUT_VPC_IDS is required for cleanup action")
	}

	// Parse space-separated VPC IDs
	vpcIDs := strings.Fields(vpcIDsStr)
	if len(vpcIDs) == 0 {
		log.Info("No VPC IDs provided, nothing to clean up")
		return nil
	}

	// Determine AWS region
	region := os.Getenv("INPUT_AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
			if region == "" {
				return fmt.Errorf(
					"AWS region must be specified via INPUT_AWS_REGION or " +
						"AWS_REGION environment variable")
			}
		}
	}

	// Check force cleanup flag
	forceCleanup := os.Getenv("INPUT_FORCE_CLEANUP") == "true"

	// Create the cleaner
	cleaner, err := cleanuppkg.New(log, region)
	if err != nil {
		return fmt.Errorf("failed to create cleaner: %w", err)
	}

	// Process each VPC ID
	successCount := 0
	failCount := 0

	for _, vpcID := range vpcIDs {
		vpcID = strings.TrimSpace(vpcID)
		if vpcID == "" {
			continue
		}

		log.Info("Processing VPC: %s", vpcID)

		var cleanupErr error
		if forceCleanup {
			// Skip job status check
			cleanupErr = cleaner.DeleteVPCResources(context.Background(), vpcID)
		} else {
			// Check job status first
			cleanupErr = cleaner.CleanupVPC(context.Background(), vpcID)
		}

		if cleanupErr != nil {
			log.Error(fmt.Errorf("failed to cleanup VPC %s: %v", vpcID, cleanupErr))
			failCount++
		} else {
			log.Info("Successfully cleaned up VPC %s", vpcID)
			successCount++
		}
	}

	if failCount > 0 {
		return fmt.Errorf(
			"cleanup completed with errors: %d succeeded, %d failed",
			successCount, failCount)
	}

	log.Info("Cleanup completed successfully: %d VPCs cleaned up", successCount)
	return nil
}
