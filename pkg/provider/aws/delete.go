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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	maxRetries        = 5
	retryDelay        = 5 * time.Second
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

func (p *Provider) delete(cache *AWS) error {
	// Phase 1: Terminate EC2 instances
	if err := p.deleteEC2Instances(cache); err != nil {
		return fmt.Errorf("failed to delete EC2 instances: %w", err)
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

			p.log.Wg.Add(1)
			go p.log.Loading("Waiting for instance %s to be terminated", id)

			ctx, cancel := context.WithTimeout(context.Background(), deletionTimeout)
			defer cancel()

			waiter := ec2.NewInstanceTerminatedWaiter(p.ec2)
			waitErr := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: []string{id},
			}, deletionTimeout)

			p.done()

			if waitErr != nil {
				// Verify if instance is actually terminated despite waiter error
				if p.isInstanceTerminated(id) {
					p.log.Info("Instance %s confirmed terminated despite waiter error", id)
					return
				}
				errChan <- fmt.Errorf("error waiting for instance %s termination: %v", id, waitErr)
				return
			}

			// Additional verification
			if !p.isInstanceTerminated(id) {
				errChan <- fmt.Errorf("instance %s not terminated after waiting", id)
				return
			}

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
	if cache.SecurityGroupid == "" {
		p.log.Info("No security group to delete")
		return nil
	}

	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Deleting security group"); err != nil {
		p.log.Error(fmt.Errorf("failed to update progressing condition: %w", err))
	}

	// Delete security group with retries
	err := p.retryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
		defer cancel()
		_, err := p.ec2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: &cache.SecurityGroupid,
		})
		if err != nil {
			// Check if security group doesn't exist
			if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
				p.log.Info("Security group %s already deleted", cache.SecurityGroupid)
				return nil
			}
			// Check if security group is still in use
			if strings.Contains(err.Error(), "DependencyViolation") {
				p.log.Info("Security group %s still in use, will retry", cache.SecurityGroupid)
				return err
			}
			return err
		}
		return nil
	})

	if err != nil {
		if err := p.updateDegradedCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Error deleting security group"); err != nil {
			p.log.Error(fmt.Errorf("failed to update degraded condition: %w", err))
		}
		return fmt.Errorf("error deleting security group %s: %w", cache.SecurityGroupid, err)
	}

	// Verify deletion
	time.Sleep(verificationDelay)
	if p.securityGroupExists(cache.SecurityGroupid) {
		return fmt.Errorf("security group %s still exists after deletion", cache.SecurityGroupid)
	}

	p.log.Info("Security group %s successfully deleted", cache.SecurityGroupid)
	return nil
}

func (p *Provider) deleteVPCResources(cache *AWS) error {
	p.log.Wg.Add(1)
	go p.log.Loading("Deleting VPC resources")
	defer p.done()

	if err := p.updateProgressingCondition(*p.DeepCopy(), cache, "v1alpha1.Destroying", "Deleting VPC resources"); err != nil {
		p.log.Error(fmt.Errorf("failed to update progressing condition: %w", err))
	}

	// Step 1: Delete Subnet
	if err := p.deleteSubnet(cache); err != nil {
		return err
	}

	// Step 2: Delete Route Table
	if err := p.deleteRouteTable(cache); err != nil {
		return err
	}

	// Step 3: Detach and Delete Internet Gateway
	if err := p.deleteInternetGateway(cache); err != nil {
		return err
	}

	// Step 4: Delete VPC
	if err := p.deleteVPC(cache); err != nil {
		return err
	}

	return p.updateTerminatedCondition(*p.Environment, cache)
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
	time.Sleep(verificationDelay)
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
		time.Sleep(verificationDelay)
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
	time.Sleep(verificationDelay * 2)

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
	time.Sleep(verificationDelay)
	if p.vpcExists(cache.Vpcid) {
		return fmt.Errorf("VPC %s still exists after deletion", cache.Vpcid)
	}

	p.log.Info("VPC %s successfully deleted", cache.Vpcid)
	return nil
}

// Helper functions

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
		time.Sleep(delay)

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
