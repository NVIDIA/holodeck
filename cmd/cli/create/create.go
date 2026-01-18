/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package create

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/instances"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/pkg/utils"

	cli "github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type options struct {
	provision      bool
	nonInteractive bool
	cachePath      string
	cacheFile      string
	envFile        string
	kubeconfig     string

	cfg   v1alpha1.Environment
	cache v1alpha1.Environment
}

// dedicatedCPThreshold is the total node count at which we suggest dedicated control-plane nodes
const dedicatedCPThreshold = 5

type command struct {
	log *logger.FunLogger
}

// NewCommand constructs the create command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	opts := options{}

	// Create the 'create' command
	create := cli.Command{
		Name:  "create",
		Usage: "create a test environment based on config file",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "provision",
				Aliases:     []string{"p"},
				Usage:       "Provision the environment",
				Destination: &opts.provision,
			},
			&cli.BoolFlag{
				Name:        "non-interactive",
				Aliases:     []string{"n"},
				Usage:       "Run in non-interactive mode (skip all prompts, use defaults)",
				Destination: &opts.nonInteractive,
				EnvVars:     []string{"HOLODECK_NONINTERACTIVE"},
			},
			&cli.StringFlag{
				Name:        "kubeconfig",
				Aliases:     []string{"k"},
				Usage:       "Path create to the kubeconfig file",
				Destination: &opts.kubeconfig,
			},
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &opts.cachePath,
			},
			&cli.StringFlag{
				Name:        "envFile",
				Aliases:     []string{"f"},
				Usage:       "Path to the Environment file",
				Destination: &opts.envFile,
			},
		},
		Before: func(c *cli.Context) error {
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.envFile)
			if err != nil {
				return fmt.Errorf("error reading config file: %s", err)
			}

			// if no containerruntime is specified, default to none
			if opts.cfg.Spec.ContainerRuntime.Name == "" {
				opts.cfg.Spec.ContainerRuntime.Name = v1alpha1.ContainerRuntimeNone
			}

			// Prompt for dedicated control-plane if threshold is met (interactive mode only)
			if shouldPromptForDedicatedCP(&opts) {
				if promptDedicatedCP(m.log) {
					opts.cfg.Spec.Cluster.ControlPlane.Dedicated = true
					m.log.Info("Control-plane nodes will be dedicated (workloads will not be scheduled on them)")
				}
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			return m.run(c, &opts)
		},
	}

	return &create
}

func (m command) run(c *cli.Context, opts *options) error {
	var provider provider.Provider
	var err error

	// Create instance manager and generate unique ID
	manager := instances.NewManager(m.log, opts.cachePath)
	instanceID := manager.GenerateInstanceID()
	opts.cacheFile = manager.GetInstanceCacheFile(instanceID)

	// Add instance ID to environment metadata
	if opts.cfg.Labels == nil {
		opts.cfg.Labels = make(map[string]string)
	}
	opts.cfg.Labels[instances.InstanceLabelKey] = instanceID
	opts.cfg.Labels[instances.InstanceProvisionedLabelKey] = "false"

	switch opts.cfg.Spec.Provider {
	case v1alpha1.ProviderAWS:
		if opts.cfg.Spec.Username == "" {
			// TODO (ArangoGutierrez): This should be based on the OS
			// Amazon Linux: ec2-user
			// Ubuntu: ubuntu
			// CentOS: centos
			// Debian: admin
			// RHEL: ec2-user
			// Fedora: ec2-user
			// SUSE: ec2-user
			opts.cfg.Spec.Username = "ubuntu"
		}
		provider, err = aws.New(m.log, opts.cfg, opts.cacheFile)
		if err != nil {
			return err
		}

	case v1alpha1.ProviderSSH:
		if opts.cfg.Spec.Username == "" {
			opts.cfg.Spec.Username = os.Getenv("USER")
		}
		m.log.Info("SSH infrastructure \u2601")
		opts.cache = opts.cfg
	}

	err = provider.Create()
	if err != nil {
		return err
	}

	// Read cache after creating the environment
	opts.cache, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.cacheFile)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	if opts.provision {
		err := runProvision(m.log, opts)
		if err != nil {
			// Handle provisioning failure with user interaction
			return m.handleProvisionFailure(instanceID, opts.cacheFile, err)
		}
	}

	// Show helpful success message with connection instructions
	m.showSuccessMessage(instanceID, opts)
	return nil
}

func (m *command) showSuccessMessage(instanceID string, opts *options) {
	// Check if this is a cluster deployment
	isCluster := opts.cfg.Spec.Cluster != nil

	if isCluster {
		m.showClusterSuccessMessage(instanceID, opts)
	} else {
		m.showSingleNodeSuccessMessage(instanceID, opts)
	}
}

func (m *command) showClusterSuccessMessage(instanceID string, opts *options) {
	cluster := opts.cfg.Spec.Cluster
	cpCount := cluster.ControlPlane.Count
	workerCount := int32(0)
	if cluster.Workers != nil {
		workerCount = cluster.Workers.Count
	}
	totalNodes := cpCount + workerCount

	m.log.Info("\n✅ Successfully created cluster: %s\n", instanceID)

	// Show cluster summary
	m.log.Info("📊 Cluster Summary:")
	m.log.Info("   Region: %s", cluster.Region)
	m.log.Info("   Control Plane Nodes: %d (%s)", cpCount, cluster.ControlPlane.InstanceType)
	if workerCount > 0 {
		m.log.Info("   Worker Nodes: %d (%s)", workerCount, cluster.Workers.InstanceType)
	}
	m.log.Info("   Total Nodes: %d\n", totalNodes)

	if cluster.HighAvailability != nil && cluster.HighAvailability.Enabled {
		m.log.Info("   High Availability: Enabled (etcd: %s)\n", cluster.HighAvailability.EtcdTopology)
	}

	// Show node list if available in status
	if opts.cache.Status.Cluster != nil && len(opts.cache.Status.Cluster.Nodes) > 0 {
		m.log.Info("📋 Cluster Nodes:")
		for _, node := range opts.cache.Status.Cluster.Nodes {
			m.log.Info("   - %s (%s): %s", node.Name, node.Role, node.PublicIP)
		}
		m.log.Info("")

		// Show SSH to first control-plane
		for _, node := range opts.cache.Status.Cluster.Nodes {
			if node.Role == "control-plane" {
				m.log.Info("📋 SSH to Control Plane:")
				m.log.Info("   ssh -i %s %s@%s\n", opts.cfg.Spec.PrivateKey, opts.cfg.Spec.Username, node.PublicIP)
				break
			}
		}
	}

	// Show kubeconfig instructions if provisioned
	if opts.cfg.Spec.Kubernetes.Install && opts.provision && opts.kubeconfig != "" {
		absPath, err := filepath.Abs(opts.kubeconfig)
		if err != nil {
			absPath = opts.kubeconfig
		}
		if _, err := os.Stat(absPath); err == nil {
			m.log.Info("📋 Kubernetes Access:")
			m.log.Info("   Kubeconfig saved to: %s", absPath)
			m.log.Info("   export KUBECONFIG=%s", absPath)
			m.log.Info("   kubectl get nodes\n")
		}
	} else if opts.cfg.Spec.Kubernetes.Install && !opts.provision {
		m.log.Info("📋 Kubernetes Access:")
		m.log.Info("   Run with --provision flag to initialize the cluster\n")
	}

	// Show next steps
	m.log.Info("📋 Next Steps:")
	m.log.Info("   - View cluster status: holodeck status %s", instanceID)
	m.log.Info("   - View live health: holodeck status %s --live", instanceID)
	m.log.Info("   - List all instances: holodeck list")
	m.log.Info("   - Delete cluster: holodeck delete %s", instanceID)
}

func (m *command) showSingleNodeSuccessMessage(instanceID string, opts *options) {
	m.log.Info("\n✅ Successfully created instance: %s\n", instanceID)

	// Get public DNS name for AWS instances
	var publicDnsName string
	if opts.cfg.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range opts.cache.Status.Properties {
			if p.Name == aws.PublicDnsName {
				publicDnsName = p.Value
				break
			}
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		publicDnsName = opts.cfg.Spec.HostUrl
	}

	// Show SSH connection instructions if we have a public DNS name
	if publicDnsName != "" && opts.cfg.Spec.Username != "" && opts.cfg.Spec.PrivateKey != "" {
		m.log.Info("📋 SSH Connection:")
		m.log.Info("   ssh -i %s %s@%s", opts.cfg.Spec.PrivateKey, opts.cfg.Spec.Username, publicDnsName)
		m.log.Info("   (If you get permission denied, run: chmod 600 %s)\n", opts.cfg.Spec.PrivateKey)
	}

	// Show kubeconfig instructions if Kubernetes was installed
	switch {
	case opts.cfg.Spec.Kubernetes.Install && opts.provision && opts.kubeconfig != "":
		// Only show kubeconfig instructions if provisioning was done and kubeconfig was requested
		absPath, err := filepath.Abs(opts.kubeconfig)
		if err != nil {
			absPath = opts.kubeconfig
		}

		// Check if the kubeconfig file actually exists
		if _, err := os.Stat(absPath); err == nil {
			m.log.Info("📋 Kubernetes Access:")
			m.log.Info("   Kubeconfig saved to: %s\n", absPath)
			m.log.Info("   Option 1 - Copy to default location:")
			m.log.Info("   cp %s ~/.kube/config\n", absPath)
			m.log.Info("   Option 2 - Set KUBECONFIG environment variable:")
			m.log.Info("   export KUBECONFIG=%s\n", absPath)
			m.log.Info("   Option 3 - Use with kubectl directly:")
			m.log.Info("   kubectl --kubeconfig=%s get nodes\n", absPath)
		}
	case opts.cfg.Spec.Kubernetes.Install && opts.provision && (opts.cfg.Spec.Kubernetes.KubernetesInstaller == "microk8s" || opts.cfg.Spec.Kubernetes.KubernetesInstaller == "kind"):
		m.log.Info("📋 Kubernetes Access:")
		m.log.Info("   Note: For %s, access kubeconfig on the instance after SSH\n", opts.cfg.Spec.Kubernetes.KubernetesInstaller)
	case opts.cfg.Spec.Kubernetes.Install && !opts.provision:
		m.log.Info("📋 Kubernetes Access:")
		m.log.Info("   Note: Run with --provision flag to install Kubernetes and download kubeconfig\n")
	}

	// Show next steps
	m.log.Info("📋 Next Steps:")
	m.log.Info("   - List instances: holodeck list")
	m.log.Info("   - Get instance status: holodeck status %s\n", instanceID)
	m.log.Info("   - Delete instance: holodeck delete %s", instanceID)
}

func (m *command) handleProvisionFailure(instanceID, cacheFile string, provisionErr error) error {
	m.log.Info("\n❌ Provisioning failed: %v\n", provisionErr)

	// Check if we're in a non-interactive environment
	if os.Getenv("CI") == "true" || os.Getenv("HOLODECK_NONINTERACTIVE") == "true" {
		m.log.Info("\n💡 To clean up the failed instance, run:")
		m.log.Info("    holodeck delete %s\n", instanceID)
		m.log.Info("💡 To list all instances:")
		m.log.Info("    holodeck list\n")
		return fmt.Errorf("provisioning failed: %w", provisionErr)
	}

	// Ask user if they want to delete the failed instance
	reader := bufio.NewReader(os.Stdin)
	m.log.Info("\n❓ Would you like to delete the failed instance? (y/N): ")

	response, err := reader.ReadString('\n')
	if err != nil {
		m.log.Info("Failed to read user input: %v", err)
		return m.provideCleanupInstructions(instanceID, provisionErr)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		// Delete the instance
		// Extract the directory path from the cache file path
		cacheDir := filepath.Dir(cacheFile)
		manager := instances.NewManager(m.log, cacheDir)
		if err := manager.DeleteInstance(instanceID); err != nil {
			m.log.Info("Failed to delete instance: %v", err)
			return m.provideCleanupInstructions(instanceID, provisionErr)
		}

		m.log.Info("✅ Successfully deleted failed instance %s\n", instanceID)
		return fmt.Errorf("provisioning failed and instance was deleted: %w", provisionErr)
	}

	return m.provideCleanupInstructions(instanceID, provisionErr)
}

func (m *command) provideCleanupInstructions(instanceID string, provisionErr error) error {
	m.log.Info("\n💡 The instance was created but provisioning failed.")
	m.log.Info("   You can manually investigate or clean up using the following commands:\n")
	m.log.Info("   To delete this specific instance:")
	m.log.Info("     holodeck delete %s\n", instanceID)
	m.log.Info("   To list all instances:")
	m.log.Info("     holodeck list\n")
	m.log.Info("   To see instance details:")
	m.log.Info("     holodeck status %s\n", instanceID)

	m.log.Info("\n💡 Additional debugging tips:")
	m.log.Info("   - Review the provisioning logs above for specific errors")
	m.log.Info("   - Check cloud provider console for instance status")
	m.log.Info("   - SSH into the instance to investigate further")

	return fmt.Errorf("provisioning failed: %w", provisionErr)
}

func runProvision(log *logger.FunLogger, opts *options) error {
	log.Info("Provisioning \u2699")

	// Copy cache status into the environment
	opts.cfg.Status = opts.cache.Status

	// Check if this is a multinode cluster
	if opts.cfg.Spec.Cluster != nil && opts.cache.Status.Cluster != nil {
		return runMultinodeProvision(log, opts)
	}

	// Single-node provisioning
	return runSingleNodeProvision(log, opts)
}

func runSingleNodeProvision(log *logger.FunLogger, opts *options) error {
	var hostUrl string

	if opts.cfg.Spec.Provider == v1alpha1.ProviderAWS {
		for _, p := range opts.cache.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
	} else if opts.cfg.Spec.Provider == v1alpha1.ProviderSSH {
		hostUrl = opts.cfg.Spec.HostUrl
	}

	p, err := provisioner.New(log, opts.cfg.Spec.PrivateKey, opts.cfg.Spec.Username, hostUrl)
	if err != nil {
		return err
	}
	defer p.Client.Close() // nolint: errcheck

	if err = p.Run(opts.cfg); err != nil {
		// Set degraded condition when provisioning fails
		opts.cfg.Status.Conditions = []metav1.Condition{
			{
				Type:               v1alpha1.ConditionDegraded,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "ProvisioningFailed",
				Message:            fmt.Sprintf("Failed to provision environment: %v", err),
			},
		}
		data, err := jyaml.MarshalYAML(opts.cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %v", err)
		}
		if err := os.WriteFile(opts.cacheFile, data, 0600); err != nil {
			return fmt.Errorf("failed to update cache file with provisioning status: %v", err)
		}
		return fmt.Errorf("failed to run provisioner: %v", err)
	}

	// Set provisioning status to true after successful provisioning
	opts.cfg.Labels[instances.InstanceProvisionedLabelKey] = "true"
	data, err := jyaml.MarshalYAML(opts.cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %v", err)
	}
	if err := os.WriteFile(opts.cacheFile, data, 0600); err != nil {
		return fmt.Errorf("failed to update cache file with provisioning status: %v", err)
	}

	// Download kubeconfig
	if opts.cfg.Spec.Kubernetes.Install && (opts.cfg.Spec.Kubernetes.KubeConfig != "" || opts.kubeconfig != "") {
		if opts.cfg.Spec.Kubernetes.KubernetesInstaller == "microk8s" || opts.cfg.Spec.Kubernetes.KubernetesInstaller == "kind" {
			log.Warning("kubeconfig retrieval is not supported for %s, skipping kubeconfig download", opts.cfg.Spec.Kubernetes.KubernetesInstaller)
			return nil
		}

		hostUrl := ""
		for _, p := range opts.cache.Status.Properties {
			if p.Name == aws.PublicDnsName {
				hostUrl = p.Value
				break
			}
		}
		if err = utils.GetKubeConfig(log, &opts.cache, hostUrl, opts.kubeconfig); err != nil {
			return fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	return nil
}

func runMultinodeProvision(log *logger.FunLogger, opts *options) error {
	log.Info("Provisioning multinode cluster...")

	// Build node list from cluster status
	var nodes []provisioner.NodeInfo
	for _, node := range opts.cache.Status.Cluster.Nodes {
		nodes = append(nodes, provisioner.NodeInfo{
			Name:      node.Name,
			PublicIP:  node.PublicIP,
			PrivateIP: node.PrivateIP,
			Role:      node.Role,
		})
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster status")
	}

	// Create cluster provisioner
	cp := provisioner.NewClusterProvisioner(
		log,
		opts.cfg.Spec.PrivateKey,
		opts.cfg.Spec.Username,
		&opts.cfg,
	)

	// Provision the cluster
	if err := cp.ProvisionCluster(nodes); err != nil {
		// Set degraded condition when provisioning fails
		opts.cfg.Status.Conditions = []metav1.Condition{
			{
				Type:               v1alpha1.ConditionDegraded,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "ProvisioningFailed",
				Message:            fmt.Sprintf("Failed to provision multinode cluster: %v", err),
			},
		}
		data, err := jyaml.MarshalYAML(opts.cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %v", err)
		}
		if err := os.WriteFile(opts.cacheFile, data, 0600); err != nil {
			return fmt.Errorf("failed to update cache file with provisioning status: %v", err)
		}
		return fmt.Errorf("failed to provision multinode cluster: %v", err)
	}

	// Set provisioning status to true after successful provisioning
	opts.cfg.Labels[instances.InstanceProvisionedLabelKey] = "true"
	data, err := jyaml.MarshalYAML(opts.cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %v", err)
	}
	if err := os.WriteFile(opts.cacheFile, data, 0600); err != nil {
		return fmt.Errorf("failed to update cache file with provisioning status: %v", err)
	}

	// Download kubeconfig from first control-plane node
	if opts.cfg.Spec.Kubernetes.Install && (opts.cfg.Spec.Kubernetes.KubeConfig != "" || opts.kubeconfig != "") {
		// Find first control-plane node
		var hostUrl string
		for _, node := range nodes {
			if node.Role == "control-plane" {
				hostUrl = node.PublicIP
				break
			}
		}
		if hostUrl != "" {
			if err := utils.GetKubeConfig(log, &opts.cache, hostUrl, opts.kubeconfig); err != nil {
				return fmt.Errorf("failed to get kubeconfig: %v", err)
			}
		}
	}

	return nil
}

// isNonInteractive checks if we should skip interactive prompts
func isNonInteractive(opts *options) bool {
	// Check CLI flag first
	if opts.nonInteractive {
		return true
	}
	// Check environment variables
	if os.Getenv("CI") == "true" {
		return true
	}
	if os.Getenv("HOLODECK_NONINTERACTIVE") == "true" {
		return true
	}
	return false
}

// shouldPromptForDedicatedCP determines if we should prompt the user about dedicated control-plane nodes
func shouldPromptForDedicatedCP(opts *options) bool {
	// Skip in non-interactive mode
	if isNonInteractive(opts) {
		return false
	}

	// Only relevant for cluster deployments
	if opts.cfg.Spec.Cluster == nil {
		return false
	}

	// Don't prompt if dedicated is already explicitly set to true
	if opts.cfg.Spec.Cluster.ControlPlane.Dedicated {
		return false
	}

	// Calculate total node count
	totalNodes := opts.cfg.Spec.Cluster.ControlPlane.Count
	if opts.cfg.Spec.Cluster.Workers != nil {
		totalNodes += opts.cfg.Spec.Cluster.Workers.Count
	}

	// Only prompt if we're at or above the threshold
	return totalNodes >= dedicatedCPThreshold
}

// promptDedicatedCP asks the user if they want dedicated control-plane nodes
func promptDedicatedCP(log *logger.FunLogger) bool {
	log.Info("")
	log.Info("╭─────────────────────────────────────────────────────────────────╮")
	log.Info("│  CLUSTER CONFIGURATION RECOMMENDATION                          │")
	log.Info("├─────────────────────────────────────────────────────────────────┤")
	log.Info("│  You're creating a cluster with %d+ nodes.                      │", dedicatedCPThreshold)
	log.Info("│  For clusters this size, dedicated control-plane nodes are     │")
	log.Info("│  recommended to ensure stability and prevent resource          │")
	log.Info("│  contention between system components and workloads.           │")
	log.Info("│                                                                 │")
	log.Info("│  When dedicated=true:                                          │")
	log.Info("│    - Control-plane nodes keep the NoSchedule taint             │")
	log.Info("│    - User workloads will only run on worker nodes              │")
	log.Info("│    - System components (etcd, API server) have dedicated       │")
	log.Info("│      resources                                                 │")
	log.Info("╰─────────────────────────────────────────────────────────────────╯")
	log.Info("")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enable dedicated control-plane nodes? [y/N]: ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
