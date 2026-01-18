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
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/tests/common"
)

// clusterTestConfig holds configuration for cluster tests
type clusterTestConfig struct {
	name        string
	filePath    string
	description string
}

// clusterTestState holds state for cluster tests
type clusterTestState struct {
	opts struct {
		cachePath string
		cachefile string
		cfg       v1alpha1.Environment
	}
	provider provider.Provider
	log      *logger.FunLogger
	ctx      context.Context
}

// AWSClusterTests contains end-to-end tests for AWS multinode cluster provisioning.
// These tests verify the complete lifecycle of multinode Kubernetes clusters, including:
// - Cluster creation with multiple nodes
// - Control plane and worker node provisioning
// - Kubernetes cluster bootstrapping with kubeadm
// - Node role and label configuration
// - Cluster health verification
// - Resource cleanup
var _ = DescribeTable("AWS Cluster E2E",
	func(config clusterTestConfig) {
		GinkgoWriter.Println("=== Starting cluster test:", config.name, "===")

		// Generate a unique artifact directory for this test
		uniqueID := common.GenerateUID()
		artifactDir := filepath.Join(LogArtifactDir, "cluster-"+config.name+"-"+uniqueID)
		Expect(os.MkdirAll(artifactDir, 0750)).To(Succeed(), "Failed to create artifact directory")

		// Setup
		state := clusterTestState{
			ctx: context.Background(),
			log: logger.NewLogger(),
		}

		// Read and validate the config file
		cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](config.filePath)
		Expect(err).NotTo(HaveOccurred(), "Failed to read config file: %s", config.filePath)

		// Validate this is a cluster config
		Expect(cfg.Spec.Cluster).NotTo(BeNil(), "Test config must have cluster spec")

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

		// --- Cluster Test Logic ---

		By("Validating cluster configuration")
		Expect(state.opts.cfg.Spec.Cluster.Region).NotTo(BeEmpty(), "Cluster region should not be empty")
		Expect(state.opts.cfg.Spec.Cluster.ControlPlane.Count).To(BeNumerically(">=", 1), "Control plane count must be at least 1")

		// Validate the cluster spec
		err = state.opts.cfg.Spec.Cluster.Validate()
		Expect(err).NotTo(HaveOccurred(), "Cluster validation failed")

		By("Creating cluster infrastructure")
		// Ensure cluster cleanup even if test fails
		DeferCleanup(func() {
			GinkgoWriter.Println("Cleaning up cluster resources...")
			Expect(state.provider.Delete()).To(Succeed(), "Failed to delete cluster")
		})

		state.opts.cfg.Spec.PrivateKey = sshKey
		state.opts.cfg.Spec.Username = "ubuntu"
		Expect(state.provider.Create()).To(Succeed(), "Failed to create cluster infrastructure")

		By("Verifying cluster status in cache")
		env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](state.opts.cachefile)
		Expect(err).NotTo(HaveOccurred(), "Failed to read environment file")
		Expect(env.Status.Cluster).NotTo(BeNil(), "Cluster status should not be nil")
		Expect(len(env.Status.Cluster.Nodes)).To(BeNumerically(">=", 1), "At least one node should be created")

		// Verify expected node count
		expectedNodes := state.opts.cfg.Spec.Cluster.ControlPlane.Count
		if state.opts.cfg.Spec.Cluster.Workers != nil {
			expectedNodes += state.opts.cfg.Spec.Cluster.Workers.Count
		}
		Expect(int32(len(env.Status.Cluster.Nodes))).To(Equal(expectedNodes), "Node count should match spec")

		// Verify control plane nodes exist
		var cpNodes, workerNodes int
		for _, node := range env.Status.Cluster.Nodes {
			switch node.Role {
			case "control-plane":
				cpNodes++
			case "worker":
				workerNodes++
			}
			Expect(node.PublicIP).NotTo(BeEmpty(), "Node public IP should not be empty")
			Expect(node.PrivateIP).NotTo(BeEmpty(), "Node private IP should not be empty")
			Expect(node.InstanceID).NotTo(BeEmpty(), "Node instance ID should not be empty")
		}
		Expect(int32(cpNodes)).To(Equal(state.opts.cfg.Spec.Cluster.ControlPlane.Count), "Control plane count should match")
		if state.opts.cfg.Spec.Cluster.Workers != nil {
			Expect(int32(workerNodes)).To(Equal(state.opts.cfg.Spec.Cluster.Workers.Count), "Worker count should match")
		}

		By("Provisioning the cluster")
		// Build nodes list for provisioner
		var nodes []provisioner.NodeInfo
		for _, node := range env.Status.Cluster.Nodes {
			nodes = append(nodes, provisioner.NodeInfo{
				Name:      node.Name,
				PublicIP:  node.PublicIP,
				PrivateIP: node.PrivateIP,
				Role:      node.Role,
			})
		}

		// Create cluster provisioner
		cp := provisioner.NewClusterProvisioner(
			state.log,
			state.opts.cfg.Spec.PrivateKey,
			state.opts.cfg.Spec.Username,
			&env,
		)

		// Provision the cluster
		Expect(cp.ProvisionCluster(nodes)).To(Succeed(), "Failed to provision cluster")

		By("Verifying cluster health")
		// Get first control-plane IP for health check
		var firstCPIP string
		for _, node := range env.Status.Cluster.Nodes {
			if node.Role == "control-plane" {
				firstCPIP = node.PublicIP
				break
			}
		}
		Expect(firstCPIP).NotTo(BeEmpty(), "First control plane IP should not be empty")

		// Check cluster health
		health, err := cp.GetClusterHealth(firstCPIP)
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster health")
		Expect(health).NotTo(BeNil(), "Cluster health should not be nil")
		Expect(health.APIServerStatus).To(Equal("Running"), "API server should be running")
		Expect(health.TotalNodes).To(Equal(int(expectedNodes)), "Total nodes should match expected")
		Expect(health.ReadyNodes).To(Equal(int(expectedNodes)), "All nodes should be ready")

		// Verify node roles in health
		Expect(health.ControlPlanes).To(Equal(cpNodes), "Control plane count in health should match")
		Expect(health.Workers).To(Equal(workerNodes), "Worker count in health should match")

		GinkgoWriter.Println("=== Finished cluster test:", config.name, "===")
	},
	Entry("3 GPU Nodes Cluster", clusterTestConfig{
		name:        "3gpu-cluster",
		filePath:    filepath.Join(packagePath, "data", "test_cluster_3gpu.yaml"),
		description: "Tests cluster with 1 non-dedicated CP + 2 GPU workers (all GPU instances)",
	}, Label("cluster", "multinode", "gpu")),
	Entry("1 CP + 3 GPU Workers", clusterTestConfig{
		name:        "1cp-3gpu-cluster",
		filePath:    filepath.Join(packagePath, "data", "test_cluster_1cp_3gpu.yaml"),
		description: "Tests cluster with 1 dedicated CPU CP + 3 GPU workers",
	}, Label("cluster", "multinode", "gpu", "dedicated")),
	Entry("HA Cluster (3 CP + 2 Workers)", clusterTestConfig{
		name:        "ha-cluster",
		filePath:    filepath.Join(packagePath, "data", "test_cluster_ha_3cp_2gpu.yaml"),
		description: "Tests HA cluster with 3 dedicated CP nodes + 2 GPU workers",
	}, Label("cluster", "multinode", "ha", "gpu")),
	Entry("Minimal Cluster (1 CP + 1 Worker)", clusterTestConfig{
		name:        "minimal-cluster",
		filePath:    filepath.Join(packagePath, "data", "test_cluster_minimal.yaml"),
		description: "Tests smallest valid multinode cluster configuration",
	}, Label("cluster", "multinode", "minimal")),
)
