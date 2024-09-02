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

package e2e

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	LogArtifactDir string
	EnvFile        string
	cwd            string
)

func TestMain(t *testing.T) {
	suiteName := "E2E Holodeck"

	RegisterFailHandler(Fail)
	RunSpecs(t,
		suiteName,
	)
}

// cleanup cleans up the test environment
func getTestEnv() {
	var err error

	LogArtifactDir = os.Getenv("LOG_ARTIFACT_DIR")
	EnvFile = os.Getenv("ENV_FILE")

	// Get current working directory
	cwd, err = os.Getwd()
	Expect(err).NotTo(HaveOccurred())
}

// BeforeSuite runs before the test suite
var _ = BeforeSuite(func() {
	// Init
	getTestEnv()
})
