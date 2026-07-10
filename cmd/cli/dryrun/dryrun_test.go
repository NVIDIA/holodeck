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

package dryrun_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "github.com/urfave/cli/v3"

	"github.com/NVIDIA/holodeck/cmd/cli/dryrun"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestDryrun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dryrun Command Suite")
}

var _ = Describe("Dryrun Command", func() {
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
			cmd := dryrun.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("dryrun"))
			Expect(cmd.Usage).To(ContainSubstring("dryrun"))
		})

		It("should have envFile flag", func() {
			cmd := dryrun.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("envFile"))
			Expect(flagNames).To(HaveKey("f"))
		})

		It("should have an action", func() {
			cmd := dryrun.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should have a before hook", func() {
			cmd := dryrun.NewCommand(log)
			Expect(cmd.Before).NotTo(BeNil())
		})
	})

	Describe("Before hook", func() {
		It("should fail when envFile does not exist", func() {
			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			// Run with non-existent env file
			err := app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", "/nonexistent/file.yaml"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read config file"))
		})

		It("should fail when envFile contains invalid YAML", func() {
			// Create temp file with invalid YAML
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "invalid.yaml")
			err = os.WriteFile(envFile, []byte("invalid: [yaml"), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read config file"))
		})

		It("should fail when provider is unknown", func() {
			// Create temp file with unknown provider
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "unknown-provider.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-env\n" +
				"spec:\n" +
				"  provider: unknown\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown provider"))
		})
	})

	Describe("SSH provider validation", func() {
		It("should fail when private key file does not exist", func() {
			// Create temp file with SSH provider but missing key
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "ssh-env.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-ssh-env\n" +
				"spec:\n" +
				"  provider: ssh\n" +
				"  auth:\n" +
				"    privateKey: /nonexistent/key\n" +
				"  instance:\n" +
				"    hostUrl: localhost\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read key file"))
		})

		It("should fail when private key is invalid", func() {
			// Create temp file with invalid key
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			keyFile := filepath.Join(tempDir, "invalid_key")
			err = os.WriteFile(keyFile, []byte("not a valid key"), 0600)
			Expect(err).NotTo(HaveOccurred())

			envFile := filepath.Join(tempDir, "ssh-env.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-ssh-env\n" +
				"spec:\n" +
				"  provider: ssh\n" +
				"  auth:\n" +
				"    privateKey: " + keyFile + "\n" +
				"  instance:\n" +
				"    hostUrl: localhost\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse private key"))
		})

		It("should use current user when username is empty", func() {
			// Create temp file with SSH provider but no username
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			keyFile := filepath.Join(tempDir, "invalid_key")
			err = os.WriteFile(keyFile, []byte("not a valid key"), 0600)
			Expect(err).NotTo(HaveOccurred())

			envFile := filepath.Join(tempDir, "ssh-env.yaml")
			// Note: no username field in spec
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-ssh-env\n" +
				"spec:\n" +
				"  provider: ssh\n" +
				"  auth:\n" +
				"    privateKey: " + keyFile + "\n" +
				"  instance:\n" +
				"    hostUrl: localhost\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			// This will fail due to invalid key, but covers the username
			// default path
			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AWS provider validation", func() {
		It("should fail when AWS provider cannot be initialized", func() {
			// Create temp file with AWS provider
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "aws-env.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-aws-env\n" +
				"spec:\n" +
				"  provider: aws\n" +
				"  instance:\n" +
				"    type: t2.micro\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			// This will fail due to AWS credentials/config issues
			// but covers the validateAWS path
			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
		})
	})

	// Cluster-mode sshConfig semantics (bastion, agent auth, per-node
	// host-key policy) are undesigned (NVIDIA/holodeck#851); the Before
	// hook must reject rather than silently ignore auth.sshConfig when
	// spec.cluster is set.
	Describe("Cluster-mode sshConfig rejection", func() {
		It("should reject a cluster env carrying auth.sshConfig before any SSH action", func() {
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "cluster-sshconfig.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-cluster-env\n" +
				"spec:\n" +
				"  provider: aws\n" +
				"  auth:\n" +
				"    sshConfig:\n" +
				"      knownHostsPolicy: accept-new\n" +
				"  cluster:\n" +
				"    region: us-west-2\n" +
				"    controlPlane:\n" +
				"      count: 1\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("auth.sshConfig is not yet supported in cluster mode (see NVIDIA/holodeck#851); remove the sshConfig block or use single-node mode"))
		})

		It("should not reject a cluster env without auth.sshConfig", func() {
			tempDir, err := os.MkdirTemp("", "holodeck-test-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, tempDir)

			envFile := filepath.Join(tempDir, "cluster-no-sshconfig.yaml")
			envContent := "apiVersion: holodeck.nvidia.com/v1alpha1\n" +
				"kind: Environment\n" +
				"metadata:\n" +
				"  name: test-cluster-env\n" +
				"spec:\n" +
				"  provider: aws\n" +
				"  cluster:\n" +
				"    region: us-west-2\n" +
				"    controlPlane:\n" +
				"      count: 1\n"
			err = os.WriteFile(envFile, []byte(envContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			cmd := dryrun.NewCommand(log)
			app := &cli.Command{
				Commands: []*cli.Command{cmd},
			}

			// May still fail later (e.g. AWS credentials), but must not be
			// the sshConfig-cluster-mode rejection.
			err = app.Run(context.Background(), []string{"holodeck", "dryrun", "-f", envFile})
			if err != nil {
				Expect(err.Error()).NotTo(ContainSubstring("not yet supported in cluster mode"))
			}
		})
	})
})
