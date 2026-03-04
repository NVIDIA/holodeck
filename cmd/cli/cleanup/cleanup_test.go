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

package cleanup_test

import (
	"bytes"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "github.com/urfave/cli/v2"

	"github.com/NVIDIA/holodeck/cmd/cli/cleanup"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestCleanup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cleanup Command Suite")
}

var _ = Describe("Cleanup Command", func() {
	var (
		log *logger.FunLogger
		buf bytes.Buffer
	)

	BeforeEach(func() {
		log = logger.NewLogger()
		log.Out = &buf
	})

	Describe("NewCommand", func() {
		It("should create a valid command", func() {
			cmd := cleanup.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("cleanup"))
			Expect(cmd.Usage).To(ContainSubstring("Clean up"))
		})

		It("should have region flag", func() {
			cmd := cleanup.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("region"))
			Expect(flagNames).To(HaveKey("r"))
		})

		It("should have force flag", func() {
			cmd := cleanup.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("force"))
			Expect(flagNames).To(HaveKey("f"))
		})

		It("should have an action", func() {
			cmd := cleanup.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should have a description", func() {
			cmd := cleanup.NewCommand(log)
			Expect(cmd.Description).NotTo(BeEmpty())
			Expect(cmd.Description).To(ContainSubstring("VPC"))
		})
	})

	Describe("Command action", func() {
		It("should require at least one VPC ID argument", func() {
			cmd := cleanup.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run without VPC ID argument
			err := app.Run([]string{"holodeck", "cleanup"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("VPC ID is required"))
		})

		It("should require region when AWS_REGION is not set", func() {
			// Clear AWS region env vars
			oldRegion := os.Getenv("AWS_REGION")
			oldDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
			Expect(os.Unsetenv("AWS_REGION")).To(Succeed())
			Expect(os.Unsetenv("AWS_DEFAULT_REGION")).To(Succeed())
			defer func() {
				if oldRegion != "" {
					Expect(os.Setenv("AWS_REGION", oldRegion)).To(Succeed())
				}
				if oldDefaultRegion != "" {
					Expect(os.Setenv("AWS_DEFAULT_REGION", oldDefaultRegion)).To(Succeed())
				}
			}()

			cmd := cleanup.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run with VPC ID but no region
			err := app.Run([]string{"holodeck", "cleanup", "vpc-12345"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("AWS region must be specified"))
		})

		It("should accept region from AWS_REGION env var", func() {
			// Set AWS region env var
			oldRegion := os.Getenv("AWS_REGION")
			Expect(os.Setenv("AWS_REGION", "us-west-2")).To(Succeed())
			defer func() {
				if oldRegion != "" {
					_ = os.Setenv("AWS_REGION", oldRegion)
				} else {
					_ = os.Unsetenv("AWS_REGION")
				}
			}()

			cmd := cleanup.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run with VPC ID - will fail at AWS connection but validates region handling
			err := app.Run([]string{"holodeck", "cleanup", "vpc-12345"})
			Expect(err).To(HaveOccurred())
			// Should fail at AWS connection, not at region validation
			Expect(err.Error()).NotTo(ContainSubstring("AWS region must be specified"))
		})

		It("should accept region from AWS_DEFAULT_REGION env var", func() {
			// Set AWS_DEFAULT_REGION env var
			oldRegion := os.Getenv("AWS_REGION")
			oldDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
			Expect(os.Unsetenv("AWS_REGION")).To(Succeed())
			Expect(os.Setenv("AWS_DEFAULT_REGION", "us-east-1")).To(Succeed())
			defer func() {
				if oldRegion != "" {
					_ = os.Setenv("AWS_REGION", oldRegion)
				}
				if oldDefaultRegion != "" {
					_ = os.Setenv("AWS_DEFAULT_REGION", oldDefaultRegion)
				} else {
					_ = os.Unsetenv("AWS_DEFAULT_REGION")
				}
			}()

			cmd := cleanup.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run with VPC ID - will fail at AWS connection but validates region handling
			err := app.Run([]string{"holodeck", "cleanup", "vpc-12345"})
			Expect(err).To(HaveOccurred())
			// Should fail at AWS connection, not at region validation
			Expect(err.Error()).NotTo(ContainSubstring("AWS region must be specified"))
		})

		It("should accept region from --region flag", func() {
			// Clear AWS region env vars
			oldRegion := os.Getenv("AWS_REGION")
			oldDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
			Expect(os.Unsetenv("AWS_REGION")).To(Succeed())
			Expect(os.Unsetenv("AWS_DEFAULT_REGION")).To(Succeed())
			defer func() {
				if oldRegion != "" {
					_ = os.Setenv("AWS_REGION", oldRegion)
				}
				if oldDefaultRegion != "" {
					_ = os.Setenv("AWS_DEFAULT_REGION", oldDefaultRegion)
				}
			}()

			cmd := cleanup.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run with --region flag - will fail at AWS connection
			err := app.Run([]string{"holodeck", "cleanup", "--region", "eu-west-1", "vpc-12345"})
			Expect(err).To(HaveOccurred())
			// Should fail at AWS connection, not at region validation
			Expect(err.Error()).NotTo(ContainSubstring("AWS region must be specified"))
		})
	})
})
