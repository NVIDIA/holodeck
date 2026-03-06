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

package delete_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "github.com/urfave/cli/v2"

	"github.com/NVIDIA/holodeck/cmd/cli/delete"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestDelete(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Delete Command Suite")
}

// sshCacheYAML returns a cache YAML for an SSH provider instance.
// SSH provider doesn't require AWS cleanup, just cache file removal.
func sshCacheYAML(instanceID, name string) string {
	return `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: ` + name + `
  labels:
    holodeck-instance-id: ` + instanceID + `
spec:
  provider: ssh
  instance:
    hostUrl: 192.168.1.100
  auth:
    keyName: test-key
    privateKey: /path/to/key.pem
    username: ubuntu
`
}

var _ = Describe("Delete Command", func() {
	var (
		log *logger.FunLogger
		buf bytes.Buffer
	)

	BeforeEach(func() {
		log = logger.NewLogger()
		log.Out = &buf
		buf.Reset()
	})

	Describe("NewCommand", func() {
		It("should create a valid command", func() {
			cmd := delete.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("delete"))
			Expect(cmd.Usage).To(ContainSubstring("Delete"))
		})

		It("should have cachepath flag", func() {
			cmd := delete.NewCommand(log)
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
		Context("with no arguments", func() {
			It("should require at least one instance ID", func() {
				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				err := app.Run([]string{"holodeck", "delete"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("at least one instance ID is required"))
			})
		})

		Context("with non-existent instance", func() {
			It("should fail when instance does not exist", func() {
				tempDir, err := os.MkdirTemp("", "holodeck-delete-test-*")
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(os.RemoveAll, tempDir)

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				err = app.Run([]string{"holodeck", "delete", "--cachepath", tempDir, "nonexistent"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get instance"))
			})
		})

		Context("with SSH provider instance", func() {
			var tempDir string

			BeforeEach(func() {
				var err error
				tempDir, err = os.MkdirTemp("", "holodeck-delete-ssh-*")
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(os.RemoveAll, tempDir)
			})

			It("should delete single SSH instance successfully", func() {
				// Create a valid SSH cache file
				yaml := sshCacheYAML("a1b2c3d4", "ssh-delete-test")
				cacheFile := filepath.Join(tempDir, "a1b2c3d4.yaml")
				err := os.WriteFile(cacheFile, []byte(yaml), 0600)
				Expect(err).NotTo(HaveOccurred())

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				err = app.Run([]string{"holodeck", "delete", "--cachepath", tempDir, "a1b2c3d4"})
				Expect(err).NotTo(HaveOccurred())

				// Verify cache file was removed
				_, err = os.Stat(cacheFile)
				Expect(os.IsNotExist(err)).To(BeTrue())

				// Verify success message
				Expect(buf.String()).To(ContainSubstring("Successfully deleted"))
				Expect(buf.String()).To(ContainSubstring("a1b2c3d4"))
			})

			It("should delete multiple SSH instances successfully", func() {
				// Create two cache files
				yaml1 := sshCacheYAML("e5f6a7b8", "ssh-multi-1")
				yaml2 := sshCacheYAML("c9d0e1f2", "ssh-multi-2")
				cacheFile1 := filepath.Join(tempDir, "e5f6a7b8.yaml")
				cacheFile2 := filepath.Join(tempDir, "c9d0e1f2.yaml")
				err := os.WriteFile(cacheFile1, []byte(yaml1), 0600)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(cacheFile2, []byte(yaml2), 0600)
				Expect(err).NotTo(HaveOccurred())

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				err = app.Run([]string{"holodeck", "delete", "--cachepath", tempDir, "e5f6a7b8", "c9d0e1f2"})
				Expect(err).NotTo(HaveOccurred())

				// Verify both cache files were removed
				_, err = os.Stat(cacheFile1)
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(cacheFile2)
				Expect(os.IsNotExist(err)).To(BeTrue())

				// Verify success messages for both
				Expect(buf.String()).To(ContainSubstring("e5f6a7b8"))
				Expect(buf.String()).To(ContainSubstring("c9d0e1f2"))
			})

			It("should stop on first error with multiple instances", func() {
				// Create only one valid cache file
				yaml := sshCacheYAML("a3b4c5d6", "ssh-valid")
				cacheFile := filepath.Join(tempDir, "a3b4c5d6.yaml")
				err := os.WriteFile(cacheFile, []byte(yaml), 0600)
				Expect(err).NotTo(HaveOccurred())

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				// First instance doesn't exist, should fail before processing second
				err = app.Run([]string{"holodeck", "delete", "--cachepath", tempDir, "nonexistent", "a3b4c5d6"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get instance nonexistent"))

				// Valid instance should still exist (not processed)
				_, err = os.Stat(cacheFile)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail if second instance doesn't exist", func() {
				// Create only one valid cache file
				yaml := sshCacheYAML("e7f8a9b0", "ssh-first")
				cacheFile := filepath.Join(tempDir, "e7f8a9b0.yaml")
				err := os.WriteFile(cacheFile, []byte(yaml), 0600)
				Expect(err).NotTo(HaveOccurred())

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				// First succeeds, second fails
				err = app.Run([]string{"holodeck", "delete", "--cachepath", tempDir, "e7f8a9b0", "nonexistent"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get instance nonexistent"))

				// First instance should be deleted
				_, err = os.Stat(cacheFile)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		Context("with custom cache path", func() {
			It("should use the provided cache path", func() {
				tempDir, err := os.MkdirTemp("", "holodeck-delete-custom-*")
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(os.RemoveAll, tempDir)

				// Create a cache file in custom path
				yaml := sshCacheYAML("f1e2d3c4", "custom-delete")
				cacheFile := filepath.Join(tempDir, "f1e2d3c4.yaml")
				err = os.WriteFile(cacheFile, []byte(yaml), 0600)
				Expect(err).NotTo(HaveOccurred())

				cmd := delete.NewCommand(log)
				app := &cli.App{
					Commands: []*cli.Command{cmd},
				}

				// Use -c alias for cachepath
				err = app.Run([]string{"holodeck", "delete", "-c", tempDir, "f1e2d3c4"})
				Expect(err).NotTo(HaveOccurred())

				// Verify cache file was removed
				_, err = os.Stat(cacheFile)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
	})
})
