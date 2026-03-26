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

package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/holodeck/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	maxRetries        = 10
	retryDelay        = 10 * time.Second
	maxRetryDelay     = 30 * time.Second
	deletionTimeout   = 15 * time.Minute
	verificationDelay = 2 * time.Second
	apiCallTimeout    = 30 * time.Second // Per-call timeout for EC2 API operations
)

// Delete deletes the EC2 instance and all associated resources
func (p *Provider) Delete() error {
	cache, err := p.unmarsalCache()
	if err != nil {
		return fmt.Errorf("error retrieving cache: %w", err)
	}

	if err := p.delete(cache); err != nil {
		return fmt.Errorf("error destroying AWS resources: %w", err)
	}

	return nil
}

// deleteNLBForCluster deletes the NLB for a cluster by looking it up by DNS name
func (p *Provider) deleteNLBForCluster(cache *ClusterCache) error {
	if cache.LoadBalancerDNS == "" {
		return nil
	}

	// Describe load balancers to find the one matching our DNS name
	ctx, cancel := context.WithTimeout(context.Background(), elbv2APITimeout)
	defer cancel()

	lbName := fmt.Sprintf("%s-nlb", p.ObjectMeta.Name)
	describeInput := &elasticloadbalancingv2.DescribeLoadBalancersInput{
		Names: []string{lbName},
	}
	describeOutput, err := p.elbv2.DescribeLoadBalancers(ctx, describeInput)
	if err != nil {
		return fmt.Errorf("error describing load balancers: %w", err)
	}

	// Find load balancer by DNS name
	for _, lb := range describeOutput.LoadBalancers {
		if aws.ToString(lb.DNSName) == cache.LoadBalancerDNS {
			cache.LoadBalancerArn = aws.ToString(lb.LoadBalancerArn)
			// Also get target group ARN if available
			// Describe target groups for this LB
			tgInput := &elasticloadbalancingv2.DescribeTargetGroupsInput{
				LoadBalancerArn: lb.LoadBalancerArn,
			}
			tgOutput, err := p.elbv2.DescribeTargetGroups(ctx, tgInput)
			if err == nil && len(tgOutput.TargetGroups) > 0 {
				cache.TargetGroupArn = aws.ToString(tgOutput.TargetGroups[0].TargetGroupArn)
			}
			return p.deleteNLB(cache)
		}
	}

	return nil
}

func (p *Provider) delete(cache *AWS) error {
	// Phase 0: Delete Load Balancer (for clusters with HA)
	if p.IsMultinode() && p.Environment.Status.Cluster != nil {
		if lbDNS := p.Environment.Status.Cluster.LoadBalancerDNS; lbDNS != "" {
			// Create ClusterCache from status to get LoadBalancerArn
			clusterCache := &ClusterCache{AWS: *cache}
			clusterCache.LoadBalancerDNS = lbDNS
			// Try to get ARN from status properties or reconstruct from DNS
			// For now, we'll need to describe the LB to get ARN
			// But we can also store it in status.Cluster properties
			// Actually, let's check if we can get it from the environment status
			if err := p.deleteNLBForCluster(clusterCache); err != nil {
				return fmt.Errorf("failed to delete load balancer (resources may be leaked): %w", err)
			}
		}
	}

	// Phase 1: Terminate EC2 instances
	if err := p.deleteEC2Instances(cache); err != nil {
		return fmt.Errorf("failed to delete EC2 instances: %w", err)
	}

	// Phase 1.5: Wait for ENIs to detach after instance termination.
	// AWS ENIs can linger for 2-5 minutes after termination, blocking SG deletion.
	if cache.Vpcid != "" {
		if err := p.waitForENIsDrained(cache.Vpcid); err != nil {
			p.log.Warning("ENI drain wait failed (continuing): %v", err)
		}
	}

	// Phase 2: Delete Security Groups
	if err := p.deleteSecurityGroups(cache); err != nil {
		return fmt.Errorf("failed to delete security groups: %w", err)
	}

	// Phase 3: Delete VPC and related resources
	if err := p.deleteVPCResources(cache); err != nil {
		return fmt.Errorf("failed to delete VPC resources: %w", err)
	}

	return nil
}

func (p *Provider) deleteEC2Instances(cache *AWS) error {
	// Collect all instance IDs to terminate
	var instanceIDs []string

	// For cluster deployments, collect all node instance IDs
	if p.Environment.Status.Cluster != nil && len(p.Environment.Status.Cluster.Nodes) > 0 {
		for _, node := range p.Environment.Status.Cluster.Nodes {
			if node.InstanceID != "" {
				instanceIDs = append(instanceIDs, node.InstanceID)
			}
		}
	}

	// Also include the single instance ID if present (for non-cluster deployments)
	if cache.Instanceid != "" {
		// Avoid duplicates
		found := false
		for _, id := range instanceIDs {
			if id == cache.Instanceid {
				found = true
				break
			}
		}
		if !found {
			instanceIDs = append(instanceIDs, cache.Instanceid)
		}
	}

	if len(instanceIDs) == 0 {
		p.log.Info("No EC2 instances to delete")
		return nil
	}

	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Terminating EC2 instances"); err != nil {
		p.log.Error(fmt.Errorf("failed to update progressing condition: %w", err))
	}

	// Terminate all instances with retries
	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			// Check if instances are already terminated
			if strings.Contains(err.Error(), "InvalidInstanceID.NotFound") {
				p.log.Info("Some instances already terminated")
				return nil
			}
			return err
		}
		return nil
	})

	if err != nil {
		if err := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Error terminating EC2 instances"); err != nil {
			p.log.Error(fmt.Errorf("failed to update degraded condition: %w", err))
		}
		return fmt.Errorf("error terminating instances: %w", err)
	}

	// Wait for all instances to terminate in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(instanceIDs))

	for _, instanceID := range instanceIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			cancelLoading := p.log.Loading("Waiting for instance %s to be terminated", id)

			ctx, cancel := context.WithTimeout(context.Background(), deletionTimeout)
			defer cancel()

			waiter := ec2.NewInstanceTerminatedWaiter(p.ec2)
			waitErr := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: []string{id},
			}, deletionTimeout)

			if waitErr != nil {
				// Verify if instance is actually terminated despite waiter error
				if p.isInstanceTerminated(id) {
					cancelLoading(nil)
					p.log.Info("Instance %s confirmed terminated despite waiter error", id)
					return
				}
				cancelLoading(logger.ErrLoadingFailed)
				errChan <- fmt.Errorf("error waiting for instance %s termination: %w", id, waitErr)
				return
			}

			// Additional verification
			if !p.isInstanceTerminated(id) {
				cancelLoading(logger.ErrLoadingFailed)
				errChan <- fmt.Errorf("instance %s not terminated after waiting", id)
				return
			}

			cancelLoading(nil)
			p.log.Info("EC2 instance %s successfully terminated", id)
		}(instanceID)
	}

	wg.Wait()
	close(errChan)

	// Collect all errors from parallel termination
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to terminate %d instance(s): %v", len(errs), errs)
	}

	return nil
}

func (p *Provider) deleteSecurityGroups(cache *AWS) error {
	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Deleting security groups"); err != nil {
		p.log.Error(fmt.Errorf("failed to update progressing condition: %w", err))
	}

	// Break cross-SG ingress references before deletion.
	// Worker SG has ingress rules referencing CP SG, and CP SG has ingress
	// rules referencing Worker SG. Both must be cleared to avoid
	// DependencyViolation errors on DeleteSecurityGroup.
	if cache.WorkerSecurityGroupid != "" && cache.CPSecurityGroupid != "" {
		if err := p.revokeSecurityGroupRules(cache.CPSecurityGroupid); err != nil {
			p.log.Warning("Error revoking CP SG %s rules (continuing): %v", cache.CPSecurityGroupid, err)
		}
		if err := p.revokeSecurityGroupRules(cache.WorkerSecurityGroupid); err != nil {
			p.log.Warning("Error revoking Worker SG %s rules (continuing): %v", cache.WorkerSecurityGroupid, err)
		}
	}

	// Delete Worker SG first — it references CP SG, so CP SG can't be deleted
	// while Worker SG still exists.
	if err := p.deleteSecurityGroup(cache.WorkerSecurityGroupid, "worker"); err != nil {
		if err := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Error deleting worker security group"); err != nil {
			p.log.Error(fmt.Errorf("failed to update degraded condition: %w", err))
		}
		return fmt.Errorf("error deleting worker security group %s: %w", cache.WorkerSecurityGroupid, err)
	}

	// Delete CP SG
	if err := p.deleteSecurityGroup(cache.CPSecurityGroupid, "control-plane"); err != nil {
		if err := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Error deleting control-plane security group"); err != nil {
			p.log.Error(fmt.Errorf("failed to update degraded condition: %w", err))
		}
		return fmt.Errorf("error deleting control-plane security group %s: %w", cache.CPSecurityGroupid, err)
	}

	// Delete the shared/single-node SG (skip if same as CP SG — cluster mode sets them equal)
	if cache.SecurityGroupid == cache.CPSecurityGroupid {
		cache.SecurityGroupid = "" // already deleted above
	}
	if err := p.deleteSecurityGroup(cache.SecurityGroupid, "shared"); err != nil {
		if err := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Error deleting security group"); err != nil {
			p.log.Error(fmt.Errorf("failed to update degraded condition: %w", err))
		}
		return fmt.Errorf("error deleting security group %s: %w", cache.SecurityGroupid, err)
	}

	return nil
}

// deleteSecurityGroup deletes a single security group by ID. It gracefully
// handles empty IDs (skips) and already-deleted groups. The label parameter
// is used only for logging (e.g. "worker", "control-plane", "shared").
func (p *Provider) deleteSecurityGroup(sgID, label string) error {
	if sgID == "" {
		p.log.Info("No %s security group to delete", label)
		return nil
	}

	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: &sgID,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
				p.log.Info("Security group %s (%s) already deleted", sgID, label)
				return nil
			}
			if strings.Contains(err.Error(), "DependencyViolation") {
				p.log.Info("Security group %s (%s) still in use, will retry", sgID, label)
				return err
			}
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Verify deletion
	p.sleep(verificationDelay)
	if p.securityGroupExists(sgID) {
		return fmt.Errorf("security group %s (%s) still exists after deletion", sgID, label)
	}

	p.log.Info("Security group %s (%s) successfully deleted", sgID, label)
	return nil
}

func (p *Provider) deleteVPCResources(cache *AWS) error {
	cancel := p.log.Loading("Deleting VPC resources")
	defer cancel(nil)

	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Deleting VPC resources"); err != nil {
		p.log.Error(fmt.Errorf("failed to update progressing condition: %w", err))
	}

	// Step 1: Delete NAT Gateway (async — must wait for deleted state before releasing EIP)
	if err := p.deleteNATGateway(cache); err != nil {
		return err
	}

	// Step 2: Release Elastic IP (must happen after NAT GW is deleted)
	if err := p.releaseElasticIP(cache); err != nil {
		return err
	}

	// Step 3: Delete public subnet (before route table — deleting the subnet
	// implicitly removes the route table association, avoiding the need for
	// ec2:DisassociateRouteTable which CI IAM may lack)
	if err := p.deletePublicSubnet(cache); err != nil {
		return err
	}

	// Step 4: Delete public route table (association removed by step 3)
	if err := p.deletePublicRouteTable(cache); err != nil {
		return err
	}

	// Step 5: Delete private Subnet
	if err := p.deleteSubnet(cache); err != nil {
		return err
	}

	// Step 6: Delete private Route Table (association removed by step 5)
	if err := p.deleteRouteTable(cache); err != nil {
		return err
	}

	// Step 7: Detach and Delete Internet Gateway
	if err := p.deleteInternetGateway(cache); err != nil {
		return err
	}

	// Step 8: Delete VPC
	if err := p.deleteVPC(cache); err != nil {
		return err
	}

	// TODO(Wave 3): Delete IAM instance profile + role when IAM client is added.
	// If cache.IAMInstanceProfileArn is populated:
	//   1. Remove role from instance profile
	//   2. Delete instance profile
	//   3. Delete the role
	// This requires adding an IAM client to the Provider struct.

	return p.updateTerminatedCondition(*p.Environment, cache)
}

func (p *Provider) deleteNATGateway(cache *AWS) error {
	if cache.NatGatewayid == "" {
		return nil
	}

	p.log.Info("Deleting NAT Gateway %s", cache.NatGatewayid)
	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()
	_, err := p.ec2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{
		NatGatewayId: &cache.NatGatewayid,
	})
	if err != nil {
		if strings.Contains(err.Error(), "NatGatewayNotFound") {
			p.log.Info("NAT Gateway %s already deleted", cache.NatGatewayid)
			return nil
		}
		return fmt.Errorf("error deleting NAT Gateway %s: %w", cache.NatGatewayid, err)
	}

	// NAT Gateway deletion is async — wait for deleted state before releasing EIP
	p.log.Info("Waiting for NAT Gateway %s to reach deleted state", cache.NatGatewayid)
	for i := 0; i < 36; i++ { // 36 × 5s = 3 minutes max
		p.sleep(5 * time.Second)
		dCtx, dCancel := context.WithTimeout(context.Background(), apiCallTimeout)
		out, err := p.ec2.DescribeNatGateways(dCtx, &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: []string{cache.NatGatewayid},
		})
		dCancel()
		if err != nil {
			if strings.Contains(err.Error(), "NatGatewayNotFound") {
				break
			}
			p.log.Warning("Error checking NAT Gateway state: %v", err)
			continue
		}
		if len(out.NatGateways) == 0 || out.NatGateways[0].State == types.NatGatewayStateDeleted {
			break
		}
	}

	p.log.Info("NAT Gateway %s deleted", cache.NatGatewayid)
	return nil
}

func (p *Provider) releaseElasticIP(cache *AWS) error {
	if cache.EIPAllocationid == "" {
		return nil
	}

	p.log.Info("Releasing Elastic IP %s", cache.EIPAllocationid)
	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()
	_, err := p.ec2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: &cache.EIPAllocationid,
	})
	if err != nil {
		if strings.Contains(err.Error(), "InvalidAllocationID.NotFound") {
			p.log.Info("Elastic IP %s already released", cache.EIPAllocationid)
			return nil
		}
		return fmt.Errorf("error releasing Elastic IP %s: %w", cache.EIPAllocationid, err)
	}

	p.log.Info("Elastic IP %s released", cache.EIPAllocationid)
	return nil
}

func (p *Provider) deletePublicRouteTable(cache *AWS) error {
	if cache.PublicRouteTable == "" {
		return nil
	}

	p.log.Info("Deleting public route table %s", cache.PublicRouteTable)
	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
			RouteTableId: &cache.PublicRouteTable,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidRouteTableID.NotFound") {
				p.log.Info("Public route table %s already deleted", cache.PublicRouteTable)
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error deleting public route table %s: %w", cache.PublicRouteTable, err)
	}

	p.log.Info("Public route table %s deleted", cache.PublicRouteTable)
	return nil
}

func (p *Provider) deletePublicSubnet(cache *AWS) error {
	if cache.PublicSubnetid == "" {
		return nil
	}

	p.log.Info("Deleting public subnet %s", cache.PublicSubnetid)
	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
			SubnetId: &cache.PublicSubnetid,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidSubnetID.NotFound") {
				p.log.Info("Public subnet %s already deleted", cache.PublicSubnetid)
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error deleting public subnet %s: %w", cache.PublicSubnetid, err)
	}

	p.log.Info("Public subnet %s deleted", cache.PublicSubnetid)
	return nil
}

func (p *Provider) deleteSubnet(cache *AWS) error {
	if cache.Subnetid == "" {
		p.log.Info("No subnet to delete")
		return nil
	}

	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
			SubnetId: &cache.Subnetid,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidSubnetID.NotFound") {
				p.log.Info("Subnet %s already deleted", cache.Subnetid)
				return nil
			}
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error deleting subnet %s: %w", cache.Subnetid, err)
	}

	// Verify deletion
	p.sleep(verificationDelay)
	if p.subnetExists(cache.Subnetid) {
		return fmt.Errorf("subnet %s still exists after deletion", cache.Subnetid)
	}

	p.log.Info("Subnet %s successfully deleted", cache.Subnetid)
	return nil
}

func (p *Provider) deleteRouteTable(cache *AWS) error {
	if cache.RouteTable == "" {
		p.log.Info("No route table to delete")
		return nil
	}

	// First, check if this is the main route table (which cannot be deleted)
	isMain, err := p.isMainRouteTable(cache.RouteTable, cache.Vpcid)
	if err != nil {
		p.log.Warning("Error checking if route table is main: %v", err)
	}
	if isMain {
		p.log.Info("Skipping deletion of main route table %s", cache.RouteTable)
		return nil
	}

	err = p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
			RouteTableId: &cache.RouteTable,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidRouteTableID.NotFound") {
				p.log.Info("Route table %s already deleted", cache.RouteTable)
				return nil
			}
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error deleting route table %s: %w", cache.RouteTable, err)
	}

	p.log.Info("Route table %s successfully deleted", cache.RouteTable)
	return nil
}

func (p *Provider) deleteInternetGateway(cache *AWS) error {
	if cache.InternetGwid == "" {
		p.log.Info("No Internet Gateway to delete")
		return nil
	}

	// Step 1: Detach Internet Gateway
	if cache.Vpcid != "" {
		err := p.retryWithBackoff(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
			defer cancel()
			_, err := p.ec2.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: &cache.InternetGwid,
				VpcId:             &cache.Vpcid,
			})
			if err != nil {
				if strings.Contains(err.Error(), "Gateway.NotAttached") {
					p.log.Info("Internet Gateway %s already detached", cache.InternetGwid)
					return nil
				}
				return err
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("error detaching Internet Gateway %s: %w", cache.InternetGwid, err)
		}

		// Wait a bit after detachment
		p.sleep(verificationDelay)
	}

	// Step 2: Delete Internet Gateway
	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: &cache.InternetGwid,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidInternetGatewayID.NotFound") {
				p.log.Info("Internet Gateway %s already deleted", cache.InternetGwid)
				return nil
			}
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error deleting Internet Gateway %s: %w", cache.InternetGwid, err)
	}

	p.log.Info("Internet Gateway %s successfully deleted", cache.InternetGwid)
	return nil
}

func (p *Provider) deleteVPC(cache *AWS) error {
	if cache.Vpcid == "" {
		p.log.Info("No VPC to delete")
		return nil
	}

	// Wait a bit before attempting VPC deletion to ensure dependencies are cleared
	p.sleep(verificationDelay * 2)

	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: &cache.Vpcid,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidVpcID.NotFound") {
				p.log.Info("VPC %s already deleted", cache.Vpcid)
				return nil
			}
			// If VPC has dependencies, list them for debugging
			if strings.Contains(err.Error(), "DependencyViolation") {
				p.log.Warning("VPC %s has dependencies, checking...", cache.Vpcid)
				p.listVPCDependencies(cache.Vpcid)
				return err
			}
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error deleting VPC %s: %w", cache.Vpcid, err)
	}

	// Verify deletion
	p.sleep(verificationDelay)
	if p.vpcExists(cache.Vpcid) {
		return fmt.Errorf("VPC %s still exists after deletion", cache.Vpcid)
	}

	p.log.Info("VPC %s successfully deleted", cache.Vpcid)
	return nil
}

// Helper functions

// waitForENIsDrained polls DescribeNetworkInterfaces until all non-available
// ENIs in the VPC are detached or deleted. This prevents DependencyViolation
// errors when deleting security groups, since AWS ENIs can linger for 2-5
// minutes after instance termination.
func (p *Provider) waitForENIsDrained(vpcID string) error {
	if vpcID == "" {
		return nil
	}

	const (
		eniPollInterval = 10 * time.Second
		eniPollTimeout  = 5 * time.Minute
	)

	deadline := time.Now().Add(eniPollTimeout)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		result, err := p.ec2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			},
		})
		cancel()

		if err != nil {
			p.log.Warning("Error checking ENIs in VPC %s: %v", vpcID, err)
		} else {
			// Count non-available ENIs (in-use ENIs block SG deletion)
			var blocking int
			for _, eni := range result.NetworkInterfaces {
				if eni.Status != types.NetworkInterfaceStatusAvailable {
					blocking++
				}
			}
			if blocking == 0 {
				p.log.Info("All ENIs in VPC %s are drained", vpcID)
				return nil
			}
			p.log.Info("Waiting for %d in-use ENI(s) in VPC %s to detach...", blocking, vpcID)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for ENIs to drain in VPC %s", vpcID)
		}

		p.sleep(eniPollInterval)
	}
}

// revokeSecurityGroupRules removes all ingress and egress rules from a security
// group before deletion. This prevents DependencyViolation errors caused by
// cross-SG references (e.g., worker SG referencing control-plane SG).
func (p *Provider) revokeSecurityGroupRules(sgID string) error {
	if sgID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()

	result, err := p.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	})
	if err != nil {
		if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
			return nil
		}
		return fmt.Errorf("error describing security group %s: %w", sgID, err)
	}

	if len(result.SecurityGroups) == 0 {
		return nil
	}

	sg := result.SecurityGroups[0]

	// Revoke all ingress rules
	if len(sg.IpPermissions) > 0 {
		p.log.Info("Revoking %d ingress rule(s) from security group %s", len(sg.IpPermissions), sgID)
		rCtx, rCancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer rCancel()
		_, err := p.ec2.RevokeSecurityGroupIngress(rCtx, &ec2.RevokeSecurityGroupIngressInput{
			GroupId:       &sgID,
			IpPermissions: sg.IpPermissions,
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
				return nil
			}
			return fmt.Errorf("error revoking ingress rules for %s: %w", sgID, err)
		}
	}

	// Note: egress rule revocation is intentionally skipped.
	// The CI IAM user (cnt-ci) lacks ec2:RevokeSecurityGroupEgress permission,
	// and the default egress rule (0.0.0.0/0) does not create cross-SG
	// dependencies that would block DeleteSecurityGroup.

	return nil
}

func (p *Provider) retryWithBackoff(operation func() error) error {
	delay := retryDelay
	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		if i == maxRetries-1 {
			return err
		}

		p.log.Info("Operation failed (attempt %d/%d): %v. Retrying in %v...", i+1, maxRetries, err, delay)
		p.sleep(delay)

		// Exponential backoff
		delay *= 2
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
	}
	return fmt.Errorf("operation failed after %d retries", maxRetries)
}

func (p *Provider) isInstanceTerminated(instanceID string) bool {
	result, err := p.ec2.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return strings.Contains(err.Error(), "InvalidInstanceID.NotFound")
	}

	if len(result.Reservations) > 0 && len(result.Reservations[0].Instances) > 0 {
		state := result.Reservations[0].Instances[0].State
		return state != nil && state.Name == types.InstanceStateNameTerminated
	}
	return true
}

func (p *Provider) securityGroupExists(sgID string) bool {
	_, err := p.ec2.DescribeSecurityGroups(context.Background(), &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	})
	return err == nil
}

func (p *Provider) subnetExists(subnetID string) bool {
	_, err := p.ec2.DescribeSubnets(context.Background(), &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	})
	return err == nil
}

func (p *Provider) vpcExists(vpcID string) bool {
	_, err := p.ec2.DescribeVpcs(context.Background(), &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	return err == nil
}

func (p *Provider) isMainRouteTable(rtID, vpcID string) (bool, error) {
	result, err := p.ec2.DescribeRouteTables(context.Background(), &ec2.DescribeRouteTablesInput{
		RouteTableIds: []string{rtID},
	})
	if err != nil {
		return false, err
	}

	if len(result.RouteTables) > 0 {
		rt := result.RouteTables[0]
		// Check if this is the main route table by looking for the association
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				return true, nil
			}
		}
	}
	return false, nil
}

func (p *Provider) listVPCDependencies(vpcID string) {
	// List remaining network interfaces
	niResult, err := p.ec2.DescribeNetworkInterfaces(context.Background(), &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err == nil && len(niResult.NetworkInterfaces) > 0 {
		p.log.Warning("Found %d network interfaces still attached to VPC:", len(niResult.NetworkInterfaces))
		for _, ni := range niResult.NetworkInterfaces {
			p.log.Warning("  - %s (Status: %s)", *ni.NetworkInterfaceId, ni.Status)
		}
	}

	// List remaining security groups
	sgResult, err := p.ec2.DescribeSecurityGroups(context.Background(), &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err == nil && len(sgResult.SecurityGroups) > 0 {
		p.log.Warning("Found %d security groups still in VPC:", len(sgResult.SecurityGroups))
		for _, sg := range sgResult.SecurityGroups {
			if sg.GroupName != nil && *sg.GroupName != "default" {
				p.log.Warning("  - %s (%s)", *sg.GroupId, *sg.GroupName)
			}
		}
	}

	// List remaining subnets
	subnetResult, err := p.ec2.DescribeSubnets(context.Background(), &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err == nil && len(subnetResult.Subnets) > 0 {
		p.log.Warning("Found %d subnets still in VPC:", len(subnetResult.Subnets))
		for _, subnet := range subnetResult.Subnets {
			p.log.Warning("  - %s", *subnet.SubnetId)
		}
	}
}
