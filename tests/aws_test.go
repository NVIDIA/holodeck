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

// AWSEnvironmentTests contains end-to-end tests for AWS environment provisioning and management.
// These tests verify the complete lifecycle of AWS environments, including:
// - Environment creation and validation
// - Kubernetes cluster setup (when enabled)
// - Resource provisioning and cleanup
// - Configuration validation
var _ = DescribeTable("AWS Environment E2E",
	func(config testConfig) {
		GinkgoWriter.Println("=== Starting test:", config.name, "===")

		// Generate a unique artifact directory for this test
		uniqueID := common.GenerateUID()
		artifactDir := filepath.Join(LogArtifactDir, config.name+"-"+uniqueID)
		Expect(os.MkdirAll(artifactDir, 0750)).To(Succeed(), "Failed to create artifact directory")

		// Setup
		state := testState{
			ctx: context.Background(),
			log: logger.NewLogger(),
		}

		// Read and validate the config file
		cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](config.filePath)
		Expect(err).NotTo(HaveOccurred(), "Failed to read config file: %s", config.filePath)

		// Set unique name for the environment
		cfg.Name = cfg.Name + "-" + uniqueID

		// Setup unique cache file
		state.opts.cachePath = artifactDir
		state.opts.cachefile = filepath.Join(state.opts.cachePath, cfg.Name+".yaml")

		// Initialize provider
		state.provider, err = newProvider(state.log, cfg, state.opts.cachefile)
		Expect(err).NotTo(HaveOccurred(), "Failed to initialize provider")
		state.opts.cfg = cfg

		// Cleanup: remove cache file and artifact dir if test passes
		DeferCleanup(func() {
			if !CurrentSpecReport().Failed() {
				err := os.RemoveAll(artifactDir)
				Expect(err).NotTo(HaveOccurred(), "Failed to remove artifact directory")
			}
		})

		// --- Test logic (copied from original) ---
		By("Configuration Validation")
		Expect(state.provider.DryRun()).To(Succeed(), "Provider validation failed")
		Expect(provisioner.Dryrun(state.log, state.opts.cfg)).To(Succeed(), "Provisioner validation failed")
		Expect(state.opts.cfg.Spec.Instance.Type).NotTo(BeEmpty(), "Instance type should not be empty")
		Expect(state.opts.cfg.Spec.Instance.Region).NotTo(BeEmpty(), "Region should not be empty")

		By("Environment Management")
		// Ensure environment cleanup even if test fails
		DeferCleanup(func() {
			Expect(state.provider.Delete()).To(Succeed(), "Failed to delete environment")
		})

		state.opts.cfg.Spec.PrivateKey = sshKey
		//nolint:staticcheck // Auth is embedded but explicit access is clearer
		state.opts.cfg.Spec.Auth.Username = "ubuntu"
		Expect(state.provider.Create()).To(Succeed(), "Failed to create environment")
		Expect(state.opts.cfg.Name).NotTo(BeEmpty(), "Environment name should not be empty")

		By("Provisioning the environment")
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
		p, err := provisioner.New(state.log, state.opts.cfg.Spec.PrivateKey, state.opts.cfg.Spec.Username, hostUrl)
		Expect(err).NotTo(HaveOccurred(), "Failed to create provisioner")
		defer func() {
			if p.Client != nil {
				session, err := p.Client.NewSession()
				if err == nil {
					session.Close() // nolint:errcheck, gosec
					if err := p.Client.Close(); err != nil {
						Expect(err).NotTo(HaveOccurred(), "Failed to close ssh client")
					}
				}
				p.Client = nil
			}
		}()
		_, runErr := p.Run(env)
		Expect(runErr).NotTo(HaveOccurred(), "Failed to provision environment")

		By("Kubernetes Configuration")
		k8s := state.opts.cfg.Spec.Kubernetes
		if k8s.Install {
			Expect(k8s.KubernetesInstaller).NotTo(BeEmpty(), "Kubernetes installer should not be empty")
			// Validate based on source type
			switch k8s.Source {
			case "git":
				Expect(k8s.Git).NotTo(BeNil(), "Git config required for git source")
				Expect(k8s.Git.Ref).NotTo(BeEmpty(), "Git ref should not be empty")
			case "latest":
				// Latest source resolves branch at provision time
				if k8s.Latest != nil {
					GinkgoWriter.Printf("Tracking branch: %s\n", k8s.Latest.Track)
				}
			default:
				// Release source (default) - check for version
				if k8s.KubernetesVersion == "" && k8s.Release == nil {
					Skip("Skipping Kubernetes validation: no version specified")
				}
			}
		} else {
			Skip("Skipping test: Kubernetes not enabled in environment file")
		}

		GinkgoWriter.Println("=== Finished test:", config.name, "===")
	},
	Entry("Default AWS Test", testConfig{
		name:        "Default AWS Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws.yml"),
		description: "Tests basic AWS environment setup with default configuration",
	}, Label("default")),
	Entry("Legacy Kubernetes Test", testConfig{
		name:        "Legacy Kubernetes Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_legacy.yml"),
		description: "Tests AWS environment with legacy Kubernetes version",
	}, Label("legacy")),
	Entry("DRA Enabled Test", testConfig{
		name:        "DRA Enabled Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_dra.yml"),
		description: "Tests AWS environment with Dynamic Resource Allocation enabled",
	}, Label("dra")),
	Entry("Kernel Features Test", testConfig{
		name:        "Kernel Features Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_kernel.yml"),
		description: "Tests AWS environment with kernel features enabled",
	}, Label("kernel")),
	Entry("CTK Git Source Test", testConfig{
		name:        "CTK Git Source Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_ctk_git.yml"),
		description: "Tests AWS environment with CTK installed from git source",
	}, Label("ctk-git")),
	Entry("K8s Git Source Test (kubeadm)", testConfig{
		name:        "K8s Git Source Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_k8s_git.yml"),
		description: "Tests AWS environment with Kubernetes built from git source using kubeadm",
	}, Label("k8s-git")),
	Entry("K8s Git Source Test (KIND)", testConfig{
		name:        "K8s KIND Git Source Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_k8s_kind_git.yml"),
		description: "Tests AWS environment with Kubernetes built from git source using KIND",
	}, Label("k8s-kind-git")),
	Entry("K8s Latest Branch Test", testConfig{
		name:        "K8s Latest Branch Test",
		filePath:    filepath.Join(packagePath, "data", "test_aws_k8s_latest.yml"),
		description: "Tests AWS environment with Kubernetes tracking master branch",
	}, Label("k8s-latest")),
)

// Note: To run tests in parallel, use: ginkgo -p or --procs=N
// GinkgoParallelProcess() returns the current parallel process index (1-indexed)
var _ = BeforeEach(func() {
	_ = GinkgoParallelProcess() // Available for debugging which process is running a test
})
