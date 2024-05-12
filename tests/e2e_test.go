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
	"flag"
	"log"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/NVIDIA/k8s-test-infra/pkg/framework"
)

var (
	LogArtifactDir = flag.String("log-artifacts", "", "Directory to store logs")
	EnvFile        = flag.String("env-file", "", "Environment file to use")
)

func TestMain(m *testing.M) {
	// Register test flags, then parse flags.
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()

	// check if flags are set and if not cancel the test run
	if *EnvFile == "" {
		log.Fatal("Required flags not set. Please set -env-file")
	}

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	// Randomize specs as well as suites
	suiteConfig.RandomizeAllSpecs = true

	ginkgo.RunSpecs(t, "nvidia holodeck e2e suite", suiteConfig, reporterConfig)
}
