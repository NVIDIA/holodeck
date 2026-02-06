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

package aws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ClusterCache extends AWS cache with multinode-specific resources
type ClusterCache struct {
	AWS

	// ControlPlaneInstances holds instance IDs for control-plane nodes
	ControlPlaneInstances []InstanceInfo
	// WorkerInstances holds instance IDs for worker nodes
	WorkerInstances []InstanceInfo

	// LoadBalancerArn is the ARN of the NLB for HA control plane
	LoadBalancerArn string
	// LoadBalancerDNS is the DNS name of the load balancer
	LoadBalancerDNS string
	// TargetGroupArn is the ARN of the target group for the load balancer
	TargetGroupArn string
}

// InstanceInfo holds information about a single instance
type InstanceInfo struct {
	InstanceID       string
	PublicIP         string
	PrivateIP        string
	PublicDNS        string
	NetworkInterface string
	Role             string // "control-plane" or "worker"
	Name             string
	SSHUsername      string // SSH username for this node's OS (e.g., "ubuntu", "ec2-user")
}

// NodeRole represents the role of a node in the cluster
type NodeRole string

const (
	NodeRoleControlPlane NodeRole = "control-plane"
	NodeRoleWorker       NodeRole = "worker"
)

// Port constants for Kubernetes cluster communication
const (
	portSSH            int32 = 22
	portHTTPS          int32 = 443
	portK8sAPI         int32 = 6443
	portKubelet        int32 = 10250
	portKubeScheduler  int32 = 10259
	portKubeController int32 = 10257
	portEtcdClient     int32 = 2379
	portEtcdPeer       int32 = 2380
	portCalicoVXLAN    int32 = 4789
	portCalicoBGP      int32 = 179
	portCalicoTypha    int32 = 5473
)

// ec2APITimeout is the timeout for EC2 API calls
const ec2APITimeout = 2 * time.Minute

// IsMultinode returns true if the environment is configured for multinode cluster
func (p *Provider) IsMultinode() bool {
	return p.Spec.Cluster != nil
}

// CreateCluster creates a multinode cluster with the specified configuration
func (p *Provider) CreateCluster() error {
	if !p.IsMultinode() {
		return fmt.Errorf("cluster spec not defined, use Create() for single-node")
	}

	cache := &ClusterCache{}

	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Creating multinode cluster resources")

	// Phase 1: Create VPC and networking (reuse existing functions)
	if err := p.createVPC(&cache.AWS); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating VPC")
		return fmt.Errorf("error creating VPC: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "VPC created")

	if err := p.createSubnet(&cache.AWS); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating subnet")
		return fmt.Errorf("error creating subnet: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Subnet created")

	if err := p.createInternetGateway(&cache.AWS); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating Internet Gateway")
		return fmt.Errorf("error creating Internet Gateway: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Internet Gateway created")

	if err := p.createRouteTable(&cache.AWS); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating route table")
		return fmt.Errorf("error creating route table: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Route Table created")

	// Phase 2: Create enhanced security group for multinode
	if err := p.createClusterSecurityGroup(cache); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating cluster security group")
		return fmt.Errorf("error creating cluster security group: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Cluster Security Group created")

	// Phase 3: Create load balancer for HA (if enabled)
	if p.isHAEnabled() {
		if err := p.createLoadBalancer(cache); err != nil {
			_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating load balancer")
			return fmt.Errorf("error creating load balancer: %w", err)
		}
		_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Load Balancer created")
	}

	// Phase 4: Create control-plane instances
	if err := p.createControlPlaneInstances(cache); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating control-plane instances")
		return fmt.Errorf("error creating control-plane instances: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Control-plane instances created")

	// Phase 5: Register control-plane instances with load balancer (if HA)
	if p.isHAEnabled() {
		if err := p.registerTargets(cache); err != nil {
			_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error registering targets")
			return fmt.Errorf("error registering load balancer targets: %w", err)
		}
	}

	// Phase 6: Create worker instances (if any)
	if p.Spec.Cluster.Workers != nil && p.Spec.Cluster.Workers.Count > 0 {
		if err := p.createWorkerInstances(cache); err != nil {
			_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error creating worker instances")
			return fmt.Errorf("error creating worker instances: %w", err)
		}
		_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Worker instances created")
	}

	// Phase 7: Disable Source/Destination Check on all instances (required for Calico)
	if err := p.disableSourceDestCheck(cache); err != nil {
		_ = p.updateDegradedCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Error disabling source/dest check")
		return fmt.Errorf("error disabling source/destination check: %w", err)
	}
	_ = p.updateProgressingCondition(*p.DeepCopy(), &cache.AWS, "v1alpha1.Creating", "Source/Destination Check disabled")

	// Update status with cluster information
	if err := p.updateClusterStatus(cache); err != nil {
		return fmt.Errorf("error updating cluster status: %w", err)
	}

	return nil
}

// isHAEnabled returns true if HA is enabled for the cluster
func (p *Provider) isHAEnabled() bool {
	return p.Spec.Cluster != nil &&
		p.Spec.Cluster.HighAvailability != nil &&
		p.Spec.Cluster.HighAvailability.Enabled
}

// createClusterSecurityGroup creates a security group with rules for multinode cluster
func (p *Provider) createClusterSecurityGroup(cache *ClusterCache) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating cluster security group")

	sgInput := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-cluster", p.ObjectMeta.Name)),
		Description: aws.String("Holodeck managed multinode cluster security group"),
		VpcId:       aws.String(cache.Vpcid),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         p.Tags,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), ec2APITimeout)
	defer cancel()
	sgOutput, err := p.ec2.CreateSecurityGroup(ctx, sgInput)
	if err != nil {
		p.fail()
		return fmt.Errorf("error creating security group: %w", err)
	}
	cache.SecurityGroupid = *sgOutput.GroupId

	// Build IP ranges for external access
	ipRangeMap := make(map[string]bool)
	ipRanges := []types.IpRange{}

	ip, err := utils.GetIPAddress()
	if err != nil {
		p.fail()
		return fmt.Errorf("error getting IP address: %w", err)
	}
	ipRangeMap[ip] = true
	ipRanges = append(ipRanges, types.IpRange{CidrIp: &ip})

	// VPC CIDR for inter-node communication
	vpcCIDR := "10.0.0.0/16"

	// Define security group rules
	permissions := []types.IpPermission{
		// SSH from user IP
		{
			FromPort:   aws.Int32(portSSH),
			ToPort:     aws.Int32(portSSH),
			IpProtocol: aws.String("tcp"),
			IpRanges:   ipRanges,
		},
		// HTTPS from user IP
		{
			FromPort:   aws.Int32(portHTTPS),
			ToPort:     aws.Int32(portHTTPS),
			IpProtocol: aws.String("tcp"),
			IpRanges:   ipRanges,
		},
		// K8s API from user IP
		{
			FromPort:   aws.Int32(portK8sAPI),
			ToPort:     aws.Int32(portK8sAPI),
			IpProtocol: aws.String("tcp"),
			IpRanges:   ipRanges,
		},
		// Kubelet API between nodes (VPC internal)
		{
			FromPort:   aws.Int32(portKubelet),
			ToPort:     aws.Int32(portKubelet),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// kube-scheduler between nodes
		{
			FromPort:   aws.Int32(portKubeScheduler),
			ToPort:     aws.Int32(portKubeScheduler),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// kube-controller-manager between nodes
		{
			FromPort:   aws.Int32(portKubeController),
			ToPort:     aws.Int32(portKubeController),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// etcd client between control-plane nodes
		{
			FromPort:   aws.Int32(portEtcdClient),
			ToPort:     aws.Int32(portEtcdClient),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// etcd peer between control-plane nodes
		{
			FromPort:   aws.Int32(portEtcdPeer),
			ToPort:     aws.Int32(portEtcdPeer),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// Calico VXLAN between nodes
		{
			FromPort:   aws.Int32(portCalicoVXLAN),
			ToPort:     aws.Int32(portCalicoVXLAN),
			IpProtocol: aws.String("udp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// Calico BGP between nodes
		{
			FromPort:   aws.Int32(portCalicoBGP),
			ToPort:     aws.Int32(portCalicoBGP),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// Calico Typha between nodes
		{
			FromPort:   aws.Int32(portCalicoTypha),
			ToPort:     aws.Int32(portCalicoTypha),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
		// K8s API between nodes (for internal communication)
		{
			FromPort:   aws.Int32(portK8sAPI),
			ToPort:     aws.Int32(portK8sAPI),
			IpProtocol: aws.String("tcp"),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(vpcCIDR)}},
		},
	}

	irInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sgOutput.GroupId,
		IpPermissions: permissions,
	}

	ctxIngress, cancelIngress := context.WithTimeout(context.Background(), ec2APITimeout)
	defer cancelIngress()
	if _, err = p.ec2.AuthorizeSecurityGroupIngress(ctxIngress, irInput); err != nil {
		p.fail()
		return fmt.Errorf("error authorizing security group ingress: %w", err)
	}

	p.done()
	return nil
}

// createControlPlaneInstances creates control-plane instances
func (p *Provider) createControlPlaneInstances(cache *ClusterCache) error {
	cpSpec := p.Spec.Cluster.ControlPlane
	count := int(cpSpec.Count)

	p.log.Wg.Add(1)
	go p.log.Loading("Creating %d control-plane instance(s)", count)

	instances, err := p.createInstances(
		cache, count, NodeRoleControlPlane,
		cpSpec.InstanceType, cpSpec.RootVolumeSizeGB,
		cpSpec.OS, cpSpec.Image,
	)
	if err != nil {
		p.fail()
		return err
	}

	cache.ControlPlaneInstances = instances
	// Set the first control-plane as the primary instance for backward compatibility
	if len(instances) > 0 {
		cache.Instanceid = instances[0].InstanceID
		cache.PublicDnsName = instances[0].PublicDNS
	}

	p.done()
	return nil
}

// createWorkerInstances creates worker instances
func (p *Provider) createWorkerInstances(cache *ClusterCache) error {
	wSpec := p.Spec.Cluster.Workers
	if wSpec == nil || wSpec.Count == 0 {
		return nil
	}

	count := int(wSpec.Count)
	p.log.Wg.Add(1)
	go p.log.Loading("Creating %d worker instance(s)", count)

	instances, err := p.createInstances(
		cache, count, NodeRoleWorker,
		wSpec.InstanceType, wSpec.RootVolumeSizeGB,
		wSpec.OS, wSpec.Image,
	)
	if err != nil {
		p.fail()
		return err
	}

	cache.WorkerInstances = instances
	p.done()
	return nil
}

// createInstances creates multiple EC2 instances with the specified role
func (p *Provider) createInstances(
	cache *ClusterCache,
	count int,
	role NodeRole,
	instanceType string,
	rootVolumeSizeGB *int32,
	os string,
	image *v1alpha1.Image,
) ([]InstanceInfo, error) {
	// Resolve AMI for this node pool
	resolved, err := p.resolveImageForNode(os, image, "")
	if err != nil {
		return nil, fmt.Errorf("error resolving AMI: %w", err)
	}
	imageID := resolved.ImageID

	// Auto-set SSH username if not already set and we got one from resolution
	//nolint:staticcheck // Auth is embedded but explicit access is clearer
	if p.Spec.Auth.Username == "" && resolved.SSHUsername != "" {
		p.Spec.Auth.Username = resolved.SSHUsername //nolint:staticcheck
	}

	// Determine volume size
	volumeSize := storageSizeGB
	if rootVolumeSizeGB != nil {
		volumeSize = *rootVolumeSizeGB
	}

	// Create instances in parallel
	var wg sync.WaitGroup
	instancesChan := make(chan InstanceInfo, count)
	errorsChan := make(chan error, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			instanceName := fmt.Sprintf("%s-%s-%d", p.ObjectMeta.Name, role, index)
			// Filter out the Name tag from p.Tags to avoid duplicates
			var tags []types.Tag
			for _, tag := range p.Tags {
				if aws.ToString(tag.Key) != "Name" {
					tags = append(tags, tag)
				}
			}
			tags = append(tags,
				types.Tag{Key: aws.String("Role"), Value: aws.String(string(role))},
				types.Tag{Key: aws.String("NodeIndex"), Value: aws.String(fmt.Sprintf("%d", index))},
				types.Tag{Key: aws.String("Name"), Value: aws.String(instanceName)},
			)

			instanceIn := &ec2.RunInstancesInput{
				ImageId:                           aws.String(imageID),
				InstanceType:                      types.InstanceType(instanceType),
				MaxCount:                          aws.Int32(1),
				MinCount:                          aws.Int32(1),
				InstanceInitiatedShutdownBehavior: types.ShutdownBehaviorTerminate,
				BlockDeviceMappings: []types.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &types.EbsBlockDevice{
							VolumeSize: aws.Int32(volumeSize),
							VolumeType: types.VolumeTypeGp2,
						},
					},
				},
				NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
					{
						AssociatePublicIpAddress: aws.Bool(true),
						DeleteOnTermination:      aws.Bool(true),
						DeviceIndex:              aws.Int32(0),
						Groups:                   []string{cache.SecurityGroupid},
						SubnetId:                 aws.String(cache.Subnetid),
					},
				},
				KeyName: aws.String(p.Spec.KeyName),
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeInstance,
						Tags:         tags,
					},
				},
			}

			ctxRun, cancelRun := context.WithTimeout(context.Background(), ec2APITimeout)
			instanceOut, err := p.ec2.RunInstances(ctxRun, instanceIn)
			cancelRun()
			if err != nil {
				errorsChan <- fmt.Errorf("error creating instance %s: %w", instanceName, err)
				return
			}

			instanceID := *instanceOut.Instances[0].InstanceId

			// Wait for instance to be running
			waiterOptions := []func(*ec2.InstanceRunningWaiterOptions){
				func(o *ec2.InstanceRunningWaiterOptions) {
					o.MaxDelay = 1 * time.Minute
					o.MinDelay = 5 * time.Second
				},
			}
			waiter := ec2.NewInstanceRunningWaiter(p.ec2, waiterOptions...)

			ctxWait, cancelWait := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancelWait()
			if err = waiter.Wait(ctxWait, &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			}, 5*time.Minute, waiterOptions...); err != nil {
				errorsChan <- fmt.Errorf("error waiting for instance %s: %w", instanceName, err)
				return
			}

			// Get instance details
			instanceRunning, err := p.ec2.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil {
				errorsChan <- fmt.Errorf("error describing instance %s: %w", instanceName, err)
				return
			}

			inst := instanceRunning.Reservations[0].Instances[0]
			info := InstanceInfo{
				InstanceID:  instanceID,
				PublicDNS:   aws.ToString(inst.PublicDnsName),
				PublicIP:    aws.ToString(inst.PublicIpAddress),
				PrivateIP:   aws.ToString(inst.PrivateIpAddress),
				Role:        string(role),
				Name:        instanceName,
				SSHUsername: resolved.SSHUsername,
			}

			if len(inst.NetworkInterfaces) > 0 {
				info.NetworkInterface = aws.ToString(inst.NetworkInterfaces[0].NetworkInterfaceId)
			}

			// Tag network interface
			if info.NetworkInterface != "" {
				ctxTags, cancelTags := context.WithTimeout(context.Background(), ec2APITimeout)
				_, err = p.ec2.CreateTags(ctxTags, &ec2.CreateTagsInput{
					Resources: []string{info.NetworkInterface},
					Tags:      tags,
				})
				cancelTags()
				if err != nil {
					p.log.Warning("Failed to tag network interface for %s: %v", instanceName, err)
				}
			}

			instancesChan <- info
		}(i)
	}

	wg.Wait()
	close(instancesChan)
	close(errorsChan)

	// Collect errors
	var errs []error
	for err := range errorsChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("errors creating instances: %v", errs)
	}

	// Collect instances
	var instances []InstanceInfo
	for info := range instancesChan {
		instances = append(instances, info)
	}

	return instances, nil
}

// disableSourceDestCheck disables Source/Destination Check on all cluster instances
// This is required for Calico networking to work correctly
// See: https://github.com/NVIDIA/holodeck/issues/586
func (p *Provider) disableSourceDestCheck(cache *ClusterCache) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Disabling Source/Destination Check for Calico networking")

	// Use explicit allocation to avoid potential slice backing array mutation
	allInstances := make([]InstanceInfo, 0, len(cache.ControlPlaneInstances)+len(cache.WorkerInstances))
	allInstances = append(allInstances, cache.ControlPlaneInstances...)
	allInstances = append(allInstances, cache.WorkerInstances...)

	for _, inst := range allInstances {
		if inst.NetworkInterface == "" {
			continue
		}

		ctxMod, cancelMod := context.WithTimeout(context.Background(), ec2APITimeout)
		_, err := p.ec2.ModifyNetworkInterfaceAttribute(ctxMod,
			&ec2.ModifyNetworkInterfaceAttributeInput{
				NetworkInterfaceId: aws.String(inst.NetworkInterface),
				SourceDestCheck: &types.AttributeBooleanValue{
					Value: aws.Bool(false),
				},
			})
		cancelMod()
		if err != nil {
			p.fail()
			return fmt.Errorf("error disabling source/dest check on %s: %w", inst.Name, err)
		}
		p.log.Info("Disabled Source/Destination Check on %s", inst.Name)
	}

	p.done()
	return nil
}

// createLoadBalancer creates an NLB for HA control plane
func (p *Provider) createLoadBalancer(cache *ClusterCache) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Creating Network Load Balancer for HA control plane")

	// Note: This requires the ELBv2 client to be added to the Provider
	// For now, we'll create a placeholder that logs a warning
	// In a full implementation, this would use the ELBv2 API

	// TODO: Add ELBv2 client to Provider and implement NLB creation
	// The implementation would:
	// 1. Create NLB in the VPC
	// 2. Create target group for port 6443
	// 3. Create listener on port 6443
	// 4. Return the NLB DNS name

	p.log.Warning("Load balancer creation not yet fully implemented - using first control-plane node as endpoint")
	p.done()
	return nil
}

// registerTargets registers control-plane instances with the load balancer target group
func (p *Provider) registerTargets(cache *ClusterCache) error {
	if cache.TargetGroupArn == "" {
		p.log.Warning("No target group ARN, skipping target registration")
		return nil
	}

	// TODO: Implement target registration when ELBv2 is fully integrated
	return nil
}

// updateClusterStatus updates the environment status with cluster information
func (p *Provider) updateClusterStatus(cache *ClusterCache) error {
	// Build node status list
	var nodes []v1alpha1.NodeStatus

	for _, inst := range cache.ControlPlaneInstances {
		nodes = append(nodes, v1alpha1.NodeStatus{
			Name:        inst.Name,
			Role:        inst.Role,
			InstanceID:  inst.InstanceID,
			PublicIP:    inst.PublicIP,
			PrivateIP:   inst.PrivateIP,
			SSHUsername: inst.SSHUsername,
			Phase:       "Ready",
		})
	}

	for _, inst := range cache.WorkerInstances {
		nodes = append(nodes, v1alpha1.NodeStatus{
			Name:        inst.Name,
			Role:        inst.Role,
			InstanceID:  inst.InstanceID,
			PublicIP:    inst.PublicIP,
			PrivateIP:   inst.PrivateIP,
			SSHUsername: inst.SSHUsername,
			Phase:       "Ready",
		})
	}

	// Update environment status
	// #nosec G115 -- node count is bounded by cluster spec, will never overflow int32
	nodeCount := int32(len(nodes))
	p.Environment.Status.Cluster = &v1alpha1.ClusterStatus{
		Nodes:                nodes,
		TotalNodes:           nodeCount,
		ReadyNodes:           nodeCount,
		Phase:                "Ready",
		ControlPlaneEndpoint: cache.PublicDnsName,
	}

	if cache.LoadBalancerDNS != "" {
		p.Environment.Status.Cluster.LoadBalancerDNS = cache.LoadBalancerDNS
		p.Environment.Status.Cluster.ControlPlaneEndpoint = cache.LoadBalancerDNS
	}

	return p.updateAvailableCondition(*p.Environment, &cache.AWS)
}

// TODO: Add ELBv2 support for HA load balancer
// This requires adding github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 to go.mod
// and implementing full NLB lifecycle management
