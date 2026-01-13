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

package list_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "github.com/urfave/cli/v2"

	"github.com/NVIDIA/holodeck/cmd/cli/list"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestList(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "List Command Suite")
}

var _ = Describe("List Command", func() {
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
			cmd := list.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("list"))
			Expect(cmd.Aliases).To(ContainElement("ls"))
			Expect(cmd.Usage).To(ContainSubstring("List"))
		})

		It("should have cachepath flag", func() {
			cmd := list.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("cachepath"))
			Expect(flagNames).To(HaveKey("c"))
		})

		It("should have quiet flag", func() {
			cmd := list.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("quiet"))
			Expect(flagNames).To(HaveKey("q"))
		})
	})

	Describe("Command action", func() {
		It("should have an action", func() {
			cmd := list.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should handle non-existent cache directory", func() {
			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Use a non-existent directory
			err := app.Run([]string{"holodeck", "list", "--cachepath", "/nonexistent/cache"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list instances"))
		})

		It("should handle empty cache directory", func() {
			// Create empty temp cache directory
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			Expect(err).NotTo(HaveOccurred())
			// Should output "No instances found"
			Expect(buf.String()).To(ContainSubstring("No instances found"))
		})

		It("should skip non-YAML files in cache directory", func() {
			// Create temp cache directory with non-YAML file
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			// Create a non-YAML file
			err = os.WriteFile(filepath.Join(tempDir, "not-yaml.txt"), []byte("test"), 0644)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			Expect(err).NotTo(HaveOccurred())
			// Should output "No instances found" since txt file is skipped
			Expect(buf.String()).To(ContainSubstring("No instances found"))
		})

		It("should handle invalid cache files gracefully", func() {
			// Create temp cache directory with invalid YAML
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			// Create an invalid YAML file
			err = os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte("invalid: [yaml"), 0644)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			// Should not fail, just skip invalid file and show warning
			err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
