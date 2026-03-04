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

package main

import (
	"fmt"
	"os"

	"github.com/NVIDIA/holodeck/cmd/action/ci"
	"github.com/NVIDIA/holodeck/internal/logger"
)

var log = logger.NewLogger()

func main() {
	gh := os.Getenv("GITHUB_ACTION")
	if gh == "" {
		log.Info("Not running in GitHub Actions, exiting")
		os.Exit(1)
	}

	// Determine action mode from INPUT_ACTION (default: create)
	action := os.Getenv("INPUT_ACTION")
	if action == "" {
		action = "create"
	}

	var err error
	switch action {
	case "create":
		err = ci.Run(log)
	case "cleanup":
		err = ci.RunCleanup(log)
	default:
		log.Error(fmt.Errorf("unknown action: %s. Valid actions: create, cleanup", action))
		os.Exit(1)
	}

	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	log.Check("Holodeck completed successfully")

	// Exit with success
	// https://docs.github.com/en/actions/creating-actions/setting-exit-codes-for-actions
	os.Exit(0)
}
