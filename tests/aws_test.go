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
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/tests/common"
)

// AWSEnvironmentTests contains end-to-end tests for AWS environment provisioning and management.
// These tests verify the complete lifecycle of AWS environments, including:
// - Environment creation and validation
// - Kubernetes cluster setup (when enabled)
// - Resource provisioning and cleanup
// - Configuration validation
var _ = Describe("AWS Environment", func() {
	// Test configuration structure
	type testConfig struct {
		name        string
		filePath    string
		description string
	}

	// Test state structure
	type testState struct {
		opts struct {
			cachePath string
			cachefile string
			cfg       v1alpha1.Environment
		}
		provider provider.Provider
		log      *logger.FunLogger
		ctx      context.Context
	}

	// Define test configurations
	testConfigs := []testConfig{
		{
			name:        "Default AWS Test",
			filePath:    filepath.Join(packagePath, "data", "test_aws.yml"),
			description: "Tests basic AWS environment setup with default configuration",
		},
		{
			name:        "Legacy Kubernetes Test",
			filePath:    filepath.Join(packagePath, "data", "test_aws_legacy.yml"),
			description: "Tests AWS environment with legacy Kubernetes version",
		},
		{
			name:        "DRA Enabled Test",
			filePath:    filepath.Join(packagePath, "data", "test_aws_dra.yml"),
			description: "Tests AWS environment with Dynamic Resource Allocation enabled",
		},
		{
			name:        "Kernel Features Test",
			filePath:    filepath.Join(packagePath, "data", "test_aws_kernel.yml"),
			description: "Tests AWS environment with kernel features enabled",
		},
	}

	// Shared setup function
	setupTest := func(config testConfig) testState {
		state := testState{
			ctx: context.Background(),
			log: logger.NewLogger(),
		}

		// Read and validate the config file
		cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](config.filePath)
		Expect(err).NotTo(HaveOccurred(), "Failed to read config file: %s", config.filePath)

		// Set unique name for the environment
		cfg.Name = cfg.Name + "-" + common.GenerateUID()

		// Setup cache directory and file
		state.opts.cachePath = LogArtifactDir
		state.opts.cachefile = filepath.Join(state.opts.cachePath, cfg.Name+".yaml")

		// Create cache directory if it doesn't exist
		Expect(os.MkdirAll(state.opts.cachePath, 0750)).To(Succeed(), "Failed to create cache directory")

		// Initialize provider
		state.provider, err = newProvider(state.log, cfg, state.opts.cachefile)
		Expect(err).NotTo(HaveOccurred(), "Failed to initialize provider")

		state.opts.cfg = cfg
		return state
	}

	// Shared cleanup function
	cleanupTest := func(state testState) {
		if !CurrentSpecReport().Failed() {
			Expect(os.Remove(state.opts.cachefile)).To(Succeed(), "Failed to remove cache file")
		}
	}

	// Run each test configuration sequentially to ensure proper resource management
	// and avoid potential conflicts between concurrent test runs
	for _, config := range testConfigs {
		config := config // Create a new variable to avoid closure issues
		When("testing "+config.name, Ordered, func() {
			var state testState

			BeforeAll(func() {
				state = setupTest(config)
			})

			AfterAll(func() {
				cleanupTest(state)
			})

			Describe("Configuration Validation", func() {
				When("validating the provider configuration", func() {
					It("should validate the provider configuration", func() {
						Expect(state.provider.DryRun()).To(Succeed(), "Provider validation failed")
					})

					It("should validate the provisioner configuration", func() {
						Expect(provisioner.Dryrun(state.log, state.opts.cfg)).To(Succeed(), "Provisioner validation failed")
					})
				})

				When("validating the environment configuration", func() {
					It("should have valid instance type", func() {
						Expect(state.opts.cfg.Spec.Instance.Type).NotTo(BeEmpty(), "Instance type should not be empty")
					})

					It("should have valid region", func() {
						Expect(state.opts.cfg.Spec.Instance.Region).NotTo(BeEmpty(), "Region should not be empty")
					})

					It("should have valid ingress IP ranges", func() {
						Expect(state.opts.cfg.Spec.Instance.IngresIpRanges).NotTo(BeEmpty(), "Ingress IP ranges should not be empty")
					})
				})
			})

			Describe("Environment Management", func() {
				When("creating the environment", func() {
					AfterAll(func() {
						// Ensure environment cleanup even if test fails
						Expect(state.provider.Delete()).To(Succeed(), "Failed to delete environment")
					})

					It("should create the environment successfully", func() {
						state.opts.cfg.Spec.PrivateKey = sshKey
						state.opts.cfg.Spec.Username = "ubuntu"
						Expect(state.provider.Create()).To(Succeed(), "Failed to create environment")
					})

					It("should have valid environment name", func() {
						Expect(state.opts.cfg.Name).NotTo(BeEmpty(), "Environment name should not be empty")
					})
					It("should provision the environment successfully", func() {
						By("Reading the environment file")
						env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](state.opts.cachefile)
						Expect(err).NotTo(HaveOccurred(), "Failed to read environment file")

						var hostUrl string
						for _, p := range env.Status.Properties {
							if p.Name == aws.PublicDnsName {
								hostUrl = p.Value
								break
							}
						}
						Expect(hostUrl).NotTo(BeEmpty(), "Host URL should not be empty")

						By("Provisioning the environment")
						p, err := provisioner.New(state.log, state.opts.cfg.Spec.PrivateKey, state.opts.cfg.Spec.Username, hostUrl)
						Expect(err).NotTo(HaveOccurred(), "Failed to create provisioner")

						// Ensure client is properly closed after test
						defer func() {
							if p.Client != nil {
								// Try to create a new session to check if connection is alive
								session, err := p.Client.NewSession()
								if err == nil {
									session.Close() // nolint:errcheck, gosec
									// Connection is alive, close it
									if err := p.Client.Close(); err != nil {
										Expect(err).NotTo(HaveOccurred(), "Failed to close ssh client")
									}
								}
								// If we get here, either the connection was already closed or we couldn't create a session
								p.Client = nil
							}
						}()

						By("Running the provisioner")
						Expect(p.Run(env)).To(Succeed(), "Failed to provision environment")
					})
				})
			})

			Describe("Kubernetes Configuration", func() {
				When("kubernetes is enabled", func() {
					BeforeEach(func() {
						if state.opts.cfg.Spec.Kubernetes.KubernetesVersion == "" {
							Skip("Skipping test: Kubernetes version not specified in environment file")
						}
					})

					It("should have valid kubernetes version", func() {
						Expect(state.opts.cfg.Spec.Kubernetes.KubernetesVersion).NotTo(BeEmpty(), "Kubernetes version should not be empty")
					})

					It("should have valid kubernetes installer", func() {
						Expect(state.opts.cfg.Spec.Kubernetes.KubernetesInstaller).NotTo(BeEmpty(), "Kubernetes installer should not be empty")
					})
				})
			})
		})
	}
})
