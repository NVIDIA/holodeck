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

package provisioner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

// ClusterProvisioner handles provisioning of multinode Kubernetes clusters
type ClusterProvisioner struct {
	log *logger.FunLogger

	// SSH credentials
	KeyPath  string
	UserName string

	// Cluster information
	Environment *v1alpha1.Environment

	// JoinToken is generated after kubeadm init and used by joining nodes
	JoinToken string
	// CertificateKey is used for control-plane joins in HA mode
	CertificateKey string
	// ControlPlaneEndpoint is the API server endpoint (LB DNS or first CP IP)
	ControlPlaneEndpoint string
	// CACertHash is the CA certificate hash for secure joins
	CACertHash string
}

// NodeInfo represents a node to be provisioned
type NodeInfo struct {
	Name        string
	PublicIP    string
	PrivateIP   string
	Role        string // "control-plane" or "worker"
	SSHUsername string // SSH username for this node (optional, falls back to ClusterProvisioner.UserName)
}

// NewClusterProvisioner creates a new cluster provisioner
func NewClusterProvisioner(log *logger.FunLogger, keyPath, userName string, env *v1alpha1.Environment) *ClusterProvisioner {
	return &ClusterProvisioner{
		log:         log,
		KeyPath:     keyPath,
		UserName:    userName,
		Environment: env,
	}
}

// getUsernameForNode returns the SSH username for a node, preferring the
// per-node username if set, otherwise falling back to the global username.
// This supports heterogeneous clusters with different OS per node pool.
func (cp *ClusterProvisioner) getUsernameForNode(node NodeInfo) string {
	if node.SSHUsername != "" {
		return node.SSHUsername
	}
	return cp.UserName
}

// ProvisionCluster provisions a multinode Kubernetes cluster
// It follows the order: init first CP → join additional CPs → join workers
func (cp *ClusterProvisioner) ProvisionCluster(nodes []NodeInfo) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes to provision")
	}

	// Separate control-plane and worker nodes
	var controlPlanes []NodeInfo
	var workers []NodeInfo
	for _, node := range nodes {
		if node.Role == "control-plane" {
			controlPlanes = append(controlPlanes, node)
		} else {
			workers = append(workers, node)
		}
	}

	if len(controlPlanes) == 0 {
		return fmt.Errorf("at least one control-plane node is required")
	}

	// Determine control plane endpoint
	// If HA with load balancer, use LB DNS; otherwise use first CP private IP
	cp.ControlPlaneEndpoint = cp.determineControlPlaneEndpoint(controlPlanes[0])

	// Phase 1: Provision base dependencies on ALL nodes in parallel
	cp.log.Info("Provisioning base dependencies on all nodes...")
	if err := cp.provisionBaseOnAllNodes(nodes); err != nil {
		return fmt.Errorf("failed to provision base dependencies: %w", err)
	}

	// Phase 2: Initialize first control-plane node
	cp.log.Info("Initializing first control-plane node: %s", controlPlanes[0].Name)
	if err := cp.initFirstControlPlane(controlPlanes[0]); err != nil {
		return fmt.Errorf("failed to initialize first control-plane: %w", err)
	}

	// Phase 3: Join additional control-plane nodes (if any)
	if len(controlPlanes) > 1 {
		for i := 1; i < len(controlPlanes); i++ {
			cp.log.Info("Joining control-plane node: %s", controlPlanes[i].Name)
			if err := cp.joinControlPlane(controlPlanes[i]); err != nil {
				return fmt.Errorf("failed to join control-plane %s: %w", controlPlanes[i].Name, err)
			}
		}
	}

	// Phase 4: Join worker nodes
	for _, worker := range workers {
		cp.log.Info("Joining worker node: %s", worker.Name)
		if err := cp.joinWorker(worker); err != nil {
			return fmt.Errorf("failed to join worker %s: %w", worker.Name, err)
		}
	}

	// Phase 5: Configure node roles, labels, and taints
	cp.log.Info("Configuring node roles and labels...")
	if err := cp.configureNodes(controlPlanes[0], nodes); err != nil {
		return fmt.Errorf("failed to configure nodes: %w", err)
	}

	cp.log.Info("Cluster provisioning complete!")
	return nil
}

// determineControlPlaneEndpoint returns the control plane endpoint
func (cp *ClusterProvisioner) determineControlPlaneEndpoint(firstCP NodeInfo) string {
	// Check if HA is enabled and we have a load balancer DNS
	if cp.Environment.Status.Cluster != nil && cp.Environment.Status.Cluster.LoadBalancerDNS != "" {
		return cp.Environment.Status.Cluster.LoadBalancerDNS
	}
	// Fall back to first control-plane private IP
	return firstCP.PrivateIP
}

// provisionBaseOnAllNodes provisions base dependencies (kernel, driver, runtime, toolkit)
// on all nodes before Kubernetes installation
func (cp *ClusterProvisioner) provisionBaseOnAllNodes(nodes []NodeInfo) error {
	// For each node, provision base dependencies (excluding Kubernetes)
	for _, node := range nodes {
		cp.log.Info("Provisioning base dependencies on %s (%s)", node.Name, node.PublicIP)

		provisioner, err := New(cp.log, cp.KeyPath, cp.getUsernameForNode(node), node.PublicIP)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
		}

		// Create a modified environment without Kubernetes install
		envCopy := cp.Environment.DeepCopy()
		envCopy.Spec.Kubernetes.Install = false

		if err := provisioner.Run(*envCopy); err != nil {
			if provisioner.Client != nil {
				_ = provisioner.Client.Close()
			}
			return fmt.Errorf("failed to provision base on %s: %w", node.Name, err)
		}
		// Client may be nil after Run() if node rebooted
		if provisioner.Client != nil {
			_ = provisioner.Client.Close()
		}
	}

	// Install Kubernetes prerequisites (kubeadm, kubelet, kubectl, CNI) on all nodes
	cp.log.Info("Installing Kubernetes prerequisites on all nodes...")
	for _, node := range nodes {
		if err := cp.installK8sPrereqs(node); err != nil {
			return fmt.Errorf("failed to install K8s prerequisites on %s: %w", node.Name, err)
		}
	}

	return nil
}

// installK8sPrereqs installs Kubernetes binaries on a node
func (cp *ClusterProvisioner) installK8sPrereqs(node NodeInfo) error {
	cp.log.Info("Installing K8s binaries on %s (%s)", node.Name, node.PublicIP)

	client, err := connectOrDie(cp.KeyPath, cp.getUsernameForNode(node), node.PublicIP)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}
	defer client.Close() // nolint: errcheck

	// Generate script
	var tpl bytes.Buffer
	if err := addScriptHeader(&tpl); err != nil {
		return fmt.Errorf("failed to add script header: %w", err)
	}

	// Generate kubeadm prerequisites script
	prereqConfig := templates.KubeadmPrereqConfig{
		Environment: cp.Environment,
	}
	if err := prereqConfig.Execute(&tpl); err != nil {
		return fmt.Errorf("failed to generate K8s prereq script: %w", err)
	}

	// Run the script via SSH
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	reader, writer := io.Pipe()
	session.Stdout = writer
	session.Stderr = writer

	// Use WaitGroup to ensure goroutine completes before returning
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, reader)
	}()

	if err := session.Start(tpl.String()); err != nil {
		_ = writer.Close() // Signal goroutine to exit
		wg.Wait()
		return fmt.Errorf("failed to start script: %w", err)
	}
	if err := session.Wait(); err != nil {
		_ = writer.Close() // Signal goroutine to exit
		wg.Wait()
		return fmt.Errorf("failed to run K8s prereq script: %w", err)
	}

	_ = writer.Close() // Signal EOF to reader
	wg.Wait()          // Wait for goroutine to finish copying
	return nil
}

// initFirstControlPlane initializes the first control-plane node with kubeadm init
func (cp *ClusterProvisioner) initFirstControlPlane(node NodeInfo) error {
	provisioner, err := New(cp.log, cp.KeyPath, cp.getUsernameForNode(node), node.PublicIP)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}
	defer provisioner.Client.Close() // nolint: errcheck

	// Set the endpoint host for kubeadm config
	cp.Environment.Spec.Kubernetes.K8sEndpointHost = cp.ControlPlaneEndpoint

	// Generate the init script
	var tpl bytes.Buffer
	if err := addScriptHeader(&tpl); err != nil {
		return fmt.Errorf("failed to add script header: %w", err)
	}

	// Generate kubeadm init script with certificate key for HA
	initConfig := templates.KubeadmInitConfig{
		Environment:          cp.Environment,
		ControlPlaneEndpoint: cp.ControlPlaneEndpoint,
		IsHA:                 cp.isHAEnabled(),
	}

	if err := initConfig.Execute(&tpl); err != nil {
		return fmt.Errorf("failed to generate kubeadm init script: %w", err)
	}

	// Run the init script
	provisioner.tpl = tpl
	if err := provisioner.provision(); err != nil {
		return fmt.Errorf("failed to run kubeadm init: %w", err)
	}

	// Extract join information from the first control-plane
	if err := cp.extractJoinInfo(provisioner); err != nil {
		return fmt.Errorf("failed to extract join info: %w", err)
	}

	return nil
}

// extractJoinInfo creates fresh join credentials from the initialized control plane.
// This always generates new tokens and re-uploads certificates to ensure full control
// over the provisioning process and enable future scale-up operations.
func (cp *ClusterProvisioner) extractJoinInfo(provisioner *Provisioner) error {
	// Step 1: Always create a fresh bootstrap token (never reuse existing)
	// This ensures we have full control and enables future cluster scaling
	cp.log.Info("Creating fresh bootstrap token...")
	session, err := provisioner.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	tokenOut, err := session.CombinedOutput("sudo kubeadm token create --ttl 2h")
	_ = session.Close()
	if err != nil {
		return fmt.Errorf("failed to create bootstrap token: %w", err)
	}
	cp.JoinToken = strings.TrimSpace(string(tokenOut))

	// Step 2: Compute CA cert hash from PKI files
	// This is deterministic based on the CA certificate
	cp.log.Info("Computing CA certificate hash...")
	session2, err := provisioner.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	hashOut, err := session2.CombinedOutput("openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //'")
	_ = session2.Close()
	if err != nil {
		return fmt.Errorf("failed to compute CA hash: %w", err)
	}
	cp.CACertHash = "sha256:" + strings.TrimSpace(string(hashOut))

	// Step 3: For HA clusters, always re-upload certificates with a fresh encryption key
	// This creates/overwrites the kubeadm-certs secret with new credentials
	if cp.isHAEnabled() {
		cp.log.Info("Uploading control-plane certificates for HA join...")
		session3, err := provisioner.Client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		// Re-upload certs to kubeadm-certs secret with a new encryption key
		certKeyOut, err := session3.CombinedOutput("sudo kubeadm init phase upload-certs --upload-certs 2>/dev/null | tail -1")
		_ = session3.Close()
		if err != nil {
			return fmt.Errorf("failed to upload certificates: %w", err)
		}
		cp.CertificateKey = strings.TrimSpace(string(certKeyOut))
	}

	cp.log.Info("Join credentials ready - Token: %s, CA Hash: %s", cp.JoinToken, cp.CACertHash[:32]+"...")
	if cp.CertificateKey != "" {
		cp.log.Info("Certificate key for control-plane joins: %s...", cp.CertificateKey[:16])
	}

	return nil
}

// joinControlPlane joins an additional control-plane node to the cluster
func (cp *ClusterProvisioner) joinControlPlane(node NodeInfo) error {
	provisioner, err := New(cp.log, cp.KeyPath, cp.getUsernameForNode(node), node.PublicIP)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}
	defer provisioner.Client.Close() // nolint: errcheck

	// Generate the join script
	var tpl bytes.Buffer
	if err := addScriptHeader(&tpl); err != nil {
		return fmt.Errorf("failed to add script header: %w", err)
	}

	joinConfig := templates.KubeadmJoinConfig{
		ControlPlaneEndpoint: cp.ControlPlaneEndpoint,
		Token:                cp.JoinToken,
		CACertHash:           cp.CACertHash,
		CertificateKey:       cp.CertificateKey,
		IsControlPlane:       true,
	}

	if err := joinConfig.Execute(&tpl); err != nil {
		return fmt.Errorf("failed to generate kubeadm join script: %w", err)
	}

	// Run the join script
	provisioner.tpl = tpl
	if err := provisioner.provision(); err != nil {
		return fmt.Errorf("failed to run kubeadm join: %w", err)
	}

	return nil
}

// joinWorker joins a worker node to the cluster
func (cp *ClusterProvisioner) joinWorker(node NodeInfo) error {
	provisioner, err := New(cp.log, cp.KeyPath, cp.getUsernameForNode(node), node.PublicIP)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", node.Name, err)
	}
	defer provisioner.Client.Close() // nolint: errcheck

	// Generate the join script
	var tpl bytes.Buffer
	if err := addScriptHeader(&tpl); err != nil {
		return fmt.Errorf("failed to add script header: %w", err)
	}

	joinConfig := templates.KubeadmJoinConfig{
		ControlPlaneEndpoint: cp.ControlPlaneEndpoint,
		Token:                cp.JoinToken,
		CACertHash:           cp.CACertHash,
		IsControlPlane:       false,
	}

	if err := joinConfig.Execute(&tpl); err != nil {
		return fmt.Errorf("failed to generate kubeadm join script: %w", err)
	}

	// Run the join script
	provisioner.tpl = tpl
	if err := provisioner.provision(); err != nil {
		return fmt.Errorf("failed to run kubeadm join: %w", err)
	}

	return nil
}

// isHAEnabled checks if HA mode is enabled
func (cp *ClusterProvisioner) isHAEnabled() bool {
	return cp.Environment.Spec.Cluster != nil &&
		cp.Environment.Spec.Cluster.HighAvailability != nil &&
		cp.Environment.Spec.Cluster.HighAvailability.Enabled
}

// configureNodes applies labels, taints, and roles to all cluster nodes
// This is run from the first control-plane node after all nodes have joined
func (cp *ClusterProvisioner) configureNodes(firstCP NodeInfo, nodes []NodeInfo) error {
	provisioner, err := New(cp.log, cp.KeyPath, cp.getUsernameForNode(firstCP), firstCP.PublicIP)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", firstCP.Name, err)
	}
	defer provisioner.Client.Close() // nolint: errcheck

	// Build the node configuration script
	// Note: Use sudo -E to preserve KUBECONFIG environment variable, or use --kubeconfig flag
	var script strings.Builder
	script.WriteString("#!/bin/bash\nset -e\n")
	script.WriteString("export KUBECONFIG=/etc/kubernetes/admin.conf\n\n")

	// Wait for all nodes to be registered
	script.WriteString(fmt.Sprintf("echo 'Waiting for all %d nodes to register...'\n", len(nodes)))
	script.WriteString("for i in {1..60}; do\n")
	script.WriteString("  NODE_COUNT=$(sudo -E kubectl get nodes --no-headers 2>/dev/null | wc -l)\n")
	script.WriteString(fmt.Sprintf("  if [ \"$NODE_COUNT\" -ge %d ]; then break; fi\n", len(nodes)))
	script.WriteString("  sleep 5\n")
	script.WriteString("done\n\n")

	// Apply holodeck managed label to all nodes
	script.WriteString("echo 'Applying holodeck managed label to all nodes...'\n")
	script.WriteString("sudo -E kubectl label nodes --all nvidia.com/holodeck.managed=true --overwrite\n\n")

	// Configure control-plane nodes
	cp.log.Info("Configuring control-plane node labels and taints...")
	cpLabels := cp.getControlPlaneLabels()
	dedicatedCP := cp.isControlPlaneDedicated()

	for _, node := range nodes {
		if node.Role != "control-plane" {
			continue
		}

		script.WriteString(fmt.Sprintf("echo 'Configuring control-plane node with IP %s...'\n", node.PrivateIP))

		// Get the actual node name from private IP
		script.WriteString(fmt.Sprintf("CP_NODE=$(sudo -E kubectl get nodes -o wide --no-headers | grep '%s' | awk '{print $1}')\n", node.PrivateIP))
		script.WriteString("if [ -n \"$CP_NODE\" ]; then\n")

		// Apply control-plane labels
		for key, value := range cpLabels {
			script.WriteString(fmt.Sprintf("  sudo -E kubectl label node \"$CP_NODE\" %s=%s --overwrite\n", key, value))
		}

		// Handle taint based on Dedicated setting
		if dedicatedCP {
			// Keep the NoSchedule taint (it's already there from kubeadm)
			script.WriteString("  echo 'Control-plane is dedicated, keeping NoSchedule taint'\n")
		} else {
			// Remove the NoSchedule taint to allow workloads
			script.WriteString("  sudo -E kubectl taint nodes \"$CP_NODE\" node-role.kubernetes.io/control-plane:NoSchedule- 2>/dev/null || true\n")
			script.WriteString("  echo 'Removed NoSchedule taint from control-plane'\n")
		}

		script.WriteString("fi\n\n")
	}

	// Configure worker nodes
	cp.log.Info("Configuring worker node labels...")
	workerLabels := cp.getWorkerLabels()

	for _, node := range nodes {
		if node.Role != "worker" {
			continue
		}

		script.WriteString(fmt.Sprintf("echo 'Configuring worker node with IP %s...'\n", node.PrivateIP))

		// Get the actual node name from private IP
		script.WriteString(fmt.Sprintf("WORKER_NODE=$(sudo -E kubectl get nodes -o wide --no-headers | grep '%s' | awk '{print $1}')\n", node.PrivateIP))
		script.WriteString("if [ -n \"$WORKER_NODE\" ]; then\n")

		// Apply worker role label
		script.WriteString("  sudo -E kubectl label node \"$WORKER_NODE\" node-role.kubernetes.io/worker= --overwrite\n")

		// Apply custom worker labels
		for key, value := range workerLabels {
			script.WriteString(fmt.Sprintf("  sudo -E kubectl label node \"$WORKER_NODE\" %s=%s --overwrite\n", key, value))
		}

		script.WriteString("fi\n\n")
	}

	// Wait for all nodes to be ready
	script.WriteString("echo 'Waiting for all nodes to be Ready...'\n")
	script.WriteString("sudo -E kubectl wait --for=condition=ready nodes --all --timeout=300s\n")
	script.WriteString("echo 'All nodes configured successfully'\n")

	// Show final node status
	script.WriteString("sudo -E kubectl get nodes -o wide\n")

	// Run the configuration script
	session, err := provisioner.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	output, err := session.CombinedOutput(script.String())
	if err != nil {
		cp.log.Info("Node configuration output: %s", string(output))
		return fmt.Errorf("failed to configure nodes: %w", err)
	}
	cp.log.Info("Node configuration output:\n%s", string(output))

	return nil
}

// getControlPlaneLabels returns labels to apply to control-plane nodes
func (cp *ClusterProvisioner) getControlPlaneLabels() map[string]string {
	labels := make(map[string]string)

	// Default labels
	labels["nvidia.com/holodeck.role"] = "control-plane"

	// Add custom labels from spec
	if cp.Environment.Spec.Cluster != nil && cp.Environment.Spec.Cluster.ControlPlane.Labels != nil {
		for k, v := range cp.Environment.Spec.Cluster.ControlPlane.Labels {
			labels[k] = v
		}
	}

	return labels
}

// getWorkerLabels returns labels to apply to worker nodes
func (cp *ClusterProvisioner) getWorkerLabels() map[string]string {
	labels := make(map[string]string)

	// Default labels
	labels["nvidia.com/holodeck.role"] = "worker"

	// Add custom labels from spec
	if cp.Environment.Spec.Cluster != nil &&
		cp.Environment.Spec.Cluster.Workers != nil &&
		cp.Environment.Spec.Cluster.Workers.Labels != nil {
		for k, v := range cp.Environment.Spec.Cluster.Workers.Labels {
			labels[k] = v
		}
	}

	return labels
}

// isControlPlaneDedicated returns true if control-plane nodes should be dedicated
// (i.e., keep the NoSchedule taint to prevent workload scheduling)
func (cp *ClusterProvisioner) isControlPlaneDedicated() bool {
	if cp.Environment.Spec.Cluster != nil {
		return cp.Environment.Spec.Cluster.ControlPlane.Dedicated
	}
	return false
}

// ClusterHealth represents the health status of a multinode cluster
type ClusterHealth struct {
	Healthy         bool
	TotalNodes      int
	ReadyNodes      int
	ControlPlanes   int
	Workers         int
	APIServerStatus string
	Nodes           []NodeHealth
	Message         string
}

// NodeHealth represents the health status of a single node
type NodeHealth struct {
	Name       string
	Role       string
	Ready      bool
	Status     string
	Version    string
	InternalIP string
}

// GetClusterHealth checks the health of a multinode cluster by querying the first control-plane
func (cp *ClusterProvisioner) GetClusterHealth(firstCPPublicIP string) (*ClusterHealth, error) {
	provisioner, err := New(cp.log, cp.KeyPath, cp.UserName, firstCPPublicIP)
	if err != nil {
		return &ClusterHealth{
			Healthy: false,
			Message: fmt.Sprintf("Failed to connect to control-plane: %v", err),
		}, nil
	}
	defer provisioner.Client.Close() // nolint: errcheck

	health := &ClusterHealth{
		Nodes: []NodeHealth{},
	}

	// Check API server status
	session, err := provisioner.Client.NewSession()
	if err != nil {
		health.Message = fmt.Sprintf("Failed to create session: %v", err)
		return health, nil
	}
	apiOut, err := session.CombinedOutput("sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf cluster-info 2>&1 | head -1")
	_ = session.Close()
	if err != nil {
		health.APIServerStatus = "Unreachable"
		health.Message = "Kubernetes API server is not responding"
		return health, nil
	}
	if strings.Contains(string(apiOut), "is running") {
		health.APIServerStatus = "Running"
	} else {
		health.APIServerStatus = "Unknown"
	}

	// Get node status
	session2, err := provisioner.Client.NewSession()
	if err != nil {
		health.Message = fmt.Sprintf("Failed to create session: %v", err)
		return health, nil
	}
	nodeOut, err := session2.CombinedOutput("sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o wide --no-headers 2>/dev/null")
	_ = session2.Close()
	if err != nil {
		health.Message = "Failed to get node status"
		return health, nil
	}

	// Parse node output
	lines := strings.Split(strings.TrimSpace(string(nodeOut)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		node := NodeHealth{
			Name:       fields[0],
			Status:     fields[1],
			Ready:      fields[1] == "Ready",
			Version:    fields[4],
			InternalIP: fields[5],
		}

		// Determine role from node name or roles column
		if len(fields) >= 3 {
			roles := fields[2]
			if strings.Contains(roles, "control-plane") {
				node.Role = "control-plane"
				health.ControlPlanes++
			} else {
				node.Role = "worker"
				health.Workers++
			}
		}

		health.Nodes = append(health.Nodes, node)
		health.TotalNodes++
		if node.Ready {
			health.ReadyNodes++
		}
	}

	// Determine overall health
	health.Healthy = health.ReadyNodes == health.TotalNodes && health.TotalNodes > 0 && health.APIServerStatus == "Running"
	switch {
	case health.Healthy:
		health.Message = fmt.Sprintf("Cluster healthy: %d/%d nodes ready", health.ReadyNodes, health.TotalNodes)
	case health.TotalNodes == 0:
		health.Message = "No nodes found in cluster"
	default:
		health.Message = fmt.Sprintf("Cluster degraded: %d/%d nodes ready", health.ReadyNodes, health.TotalNodes)
	}

	return health, nil
}

// GetClusterHealthFromEnv gets cluster health using environment configuration
func GetClusterHealthFromEnv(log *logger.FunLogger, env *v1alpha1.Environment) (*ClusterHealth, error) {
	if env.Spec.Cluster == nil || env.Status.Cluster == nil {
		return nil, fmt.Errorf("not a multinode cluster")
	}

	// Find first control-plane node
	var firstCPIP string
	for _, node := range env.Status.Cluster.Nodes {
		if node.Role == "control-plane" {
			firstCPIP = node.PublicIP
			break
		}
	}
	if firstCPIP == "" {
		return nil, fmt.Errorf("no control-plane node found")
	}

	cp := NewClusterProvisioner(log, env.Spec.PrivateKey, env.Spec.Username, env)
	return cp.GetClusterHealth(firstCPIP)
}

// Note: addScriptHeader is defined in provisioner.go
