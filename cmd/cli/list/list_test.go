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
	"io"
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

// cacheYAMLWithLabel returns a cache YAML with the holodeck-instance-id label
// required for the list command to recognize the instance.
func cacheYAMLWithLabel(instanceID, name, provider string) string {
	return `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: ` + name + `
  labels:
    holodeck-instance-id: ` + instanceID + `
spec:
  provider: ` + provider + `
  instance:
    type: t3.medium
    region: us-east-1
    image:
      architecture: amd64
  auth:
    keyName: test-key
    privateKey: /path/to/key.pem
    username: ubuntu
`
}

// cacheYAMLWithoutLabel returns a cache YAML without the instance label.
func cacheYAMLWithoutLabel(name, provider string) string {
	return `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: ` + name + `
spec:
  provider: ` + provider + `
  instance:
    type: t3.medium
    region: us-east-1
    image:
      architecture: amd64
  auth:
    keyName: test-key
    privateKey: /path/to/key.pem
    username: ubuntu
`
}

// captureStdout captures stdout during function execution.
func captureStdout(fn func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

var _ = Describe("List Command", func() {
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
			DeferCleanup(os.RemoveAll, tempDir)

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
			DeferCleanup(os.RemoveAll, tempDir)

			// Create a non-YAML file
			err = os.WriteFile(filepath.Join(tempDir, "not-yaml.txt"), []byte("test"), 0600)
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
			DeferCleanup(os.RemoveAll, tempDir)

			// Create an invalid YAML file
			err = os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte("invalid: [yaml"), 0600)
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

	Describe("Table output", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "holodeck-list-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)
		})

		It("should display table header and instance data", func() {
			// Create a valid cache file with instance label
			yaml := cacheYAMLWithLabel("abc12345", "test-instance", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "abc12345.yaml"), []byte(yaml), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			})
			Expect(err).NotTo(HaveOccurred())
			// Table header
			Expect(stdout).To(ContainSubstring("INSTANCE ID"))
			Expect(stdout).To(ContainSubstring("NAME"))
			Expect(stdout).To(ContainSubstring("PROVIDER"))
			Expect(stdout).To(ContainSubstring("STATUS"))
			// Instance data
			Expect(stdout).To(ContainSubstring("abc12345"))
			Expect(stdout).To(ContainSubstring("test-instance"))
			Expect(stdout).To(ContainSubstring("ssh"))
		})

		It("should display multiple instances", func() {
			// Create two cache files
			yaml1 := cacheYAMLWithLabel("inst0001", "instance-one", "ssh")
			yaml2 := cacheYAMLWithLabel("inst0002", "instance-two", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "inst0001.yaml"), []byte(yaml1), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "inst0002.yaml"), []byte(yaml2), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			})
			Expect(err).NotTo(HaveOccurred())
			// Both instances should appear
			Expect(stdout).To(ContainSubstring("inst0001"))
			Expect(stdout).To(ContainSubstring("instance-one"))
			Expect(stdout).To(ContainSubstring("inst0002"))
			Expect(stdout).To(ContainSubstring("instance-two"))
		})

		It("should skip instances without ID label", func() {
			// Create a cache file without the instance label
			yaml := cacheYAMLWithoutLabel("no-id-instance", "ssh")
			// Use a short filename (not UUID-length)
			err := os.WriteFile(filepath.Join(tempDir, "noid.yaml"), []byte(yaml), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir})
			Expect(err).NotTo(HaveOccurred())
			// Should output "No instances found" since instance has no ID
			Expect(buf.String()).To(ContainSubstring("No instances found"))
		})
	})

	Describe("Quiet mode", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "holodeck-list-quiet-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)
		})

		It("should only print instance IDs with -q flag", func() {
			// Create a valid cache file
			yaml := cacheYAMLWithLabel("quietid01", "quiet-test", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "quietid01.yaml"), []byte(yaml), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir, "-q"})
			})
			Expect(err).NotTo(HaveOccurred())
			// Should only have instance ID, not table headers
			Expect(stdout).To(ContainSubstring("quietid01"))
			Expect(stdout).NotTo(ContainSubstring("INSTANCE ID"))
			Expect(stdout).NotTo(ContainSubstring("NAME"))
		})

		It("should only print instance IDs with --quiet flag", func() {
			// Create a valid cache file
			yaml := cacheYAMLWithLabel("quietid02", "quiet-test-2", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "quietid02.yaml"), []byte(yaml), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir, "--quiet"})
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("quietid02"))
			Expect(stdout).NotTo(ContainSubstring("INSTANCE ID"))
		})

		It("should print multiple IDs in quiet mode", func() {
			// Create two cache files
			yaml1 := cacheYAMLWithLabel("qmulti01", "multi-1", "ssh")
			yaml2 := cacheYAMLWithLabel("qmulti02", "multi-2", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "qmulti01.yaml"), []byte(yaml1), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "qmulti02.yaml"), []byte(yaml2), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir, "-q"})
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("qmulti01"))
			Expect(stdout).To(ContainSubstring("qmulti02"))
		})

		It("should skip instances without ID in quiet mode", func() {
			// Create one valid and one without label
			yaml1 := cacheYAMLWithLabel("validq01", "valid-quiet", "ssh")
			yaml2 := cacheYAMLWithoutLabel("no-id-quiet", "ssh")
			err := os.WriteFile(filepath.Join(tempDir, "validq01.yaml"), []byte(yaml1), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "noid2.yaml"), []byte(yaml2), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			stdout := captureStdout(func() {
				err = app.Run([]string{"holodeck", "list", "--cachepath", tempDir, "-q"})
			})
			Expect(err).NotTo(HaveOccurred())
			// Only valid instance should appear
			Expect(stdout).To(ContainSubstring("validq01"))
			Expect(stdout).NotTo(ContainSubstring("no-id-quiet"))
		})
	})

	Describe("Command alias", func() {
		It("should work with 'ls' alias", func() {
			tempDir, err := os.MkdirTemp("", "holodeck-ls-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			cmd := list.NewCommand(log)
			app := &cli.App{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run([]string{"holodeck", "ls", "--cachepath", tempDir})
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("No instances found"))
		})
	})
})
