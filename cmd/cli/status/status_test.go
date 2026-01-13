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

package status_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "github.com/urfave/cli/v2"

	"github.com/NVIDIA/holodeck/cmd/cli/status"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestStatus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Command Suite")
}

var _ = Describe("Status Command", func() {
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
			cmd := status.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("status"))
			Expect(cmd.Usage).NotTo(BeEmpty())
		})

		It("should have cachepath flag", func() {
			cmd := status.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("cachepath"))
			Expect(flagNames).To(HaveKey("c"))
		})
	})

	Describe("Command action", func() {
		It("should have an action", func() {
			cmd := status.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should require instance ID argument", func() {
			cmd := status.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run without instance ID argument
			err := app.Run([]string{"holodeck", "status"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("instance ID is required"))
		})

		It("should accept instance ID argument", func() {
			// Create temp cache directory
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			cmd := status.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Run with instance ID but non-existent cache
			err = app.Run([]string{"holodeck", "status", "--cachepath", tempDir, "test-instance"})
			Expect(err).To(HaveOccurred())
			// Should fail because cache file doesn't exist, but validates arg handling
			Expect(err.Error()).To(ContainSubstring("failed to get instance"))
		})

		It("should handle invalid cache file gracefully", func() {
			// Create temp cache directory with invalid cache file
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			// Create an invalid cache file
			cacheFile := filepath.Join(tempDir, "test-instance.yaml")
			err = os.WriteFile(cacheFile, []byte("invalid: [yaml"), 0644)
			Expect(err).NotTo(HaveOccurred())

			cmd := status.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run([]string{"holodeck", "status", "--cachepath", tempDir, "test-instance"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to"))
		})
	})
})
