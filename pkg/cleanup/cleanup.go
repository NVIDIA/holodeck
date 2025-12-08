/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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

package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/NVIDIA/holodeck/internal/logger"
)

// Cleaner handles cleanup of AWS resources
type Cleaner struct {
	ec2 *ec2.Client
	log *logger.FunLogger
}

// New creates a new AWS resource cleaner
func New(log *logger.FunLogger, region string) (*Cleaner, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Cleaner{
		ec2: ec2.NewFromConfig(cfg),
		log: log,
	}, nil
}

// GetTagValue retrieves a tag value for a given VPC
func (c *Cleaner) GetTagValue(vpcID, key string) (string, error) {
	input := &ec2.DescribeTagsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("key"),
				Values: []string{key},
			},
		},
	}

	result, err := c.ec2.DescribeTags(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to describe tags: %w", err)
	}

	if len(result.Tags) == 0 {
		return "", nil
	}

	return *result.Tags[0].Value, nil
}

// GitHubJobsResponse represents the GitHub API response for job status
type GitHubJobsResponse struct {
	Jobs []struct {
		Status string `json:"status"`
	} `json:"jobs"`
}

// CheckGitHubJobsCompleted checks if all GitHub jobs are completed
func (c *Cleaner) CheckGitHubJobsCompleted(repository, runID, token string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%s/jobs", repository, runID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Warning("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		// If not found, consider it safe to delete
		return true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var jobsResp GitHubJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobsResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if all jobs are completed
	for _, job := range jobsResp.Jobs {
		if job.Status != "completed" {
			return false, nil
		}
	}

	return true, nil
}

// CleanupVPC checks job status and deletes VPC resources if jobs are completed
func (c *Cleaner) CleanupVPC(vpcID string) error {
	// Get GitHub-related tags
	repository, err := c.GetTagValue(vpcID, "GitHubRepository")
	if err != nil {
		return fmt.Errorf("failed to get GitHubRepository tag: %w", err)
	}

	runID, err := c.GetTagValue(vpcID, "GitHubRunId")
	if err != nil {
		return fmt.Errorf("failed to get GitHubRunId tag: %w", err)
	}

	// Get GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		c.log.Warning("GITHUB_TOKEN not set, skipping job status check")
	} else if repository != "" && runID != "" {
		// Check if jobs are completed
		completed, err := c.CheckGitHubJobsCompleted(repository, runID, token)
		if err != nil {
			c.log.Warning("Failed to check GitHub job status: %v", err)
		} else if !completed {
			return fmt.Errorf("GitHub jobs are still running for VPC %s", vpcID)
		}
	}

	c.log.Info("All jobs completed or no job information found, proceeding with cleanup of VPC %s", vpcID)
	return c.DeleteVPCResources(vpcID)
}

// DeleteVPCResources deletes all resources associated with a VPC
func (c *Cleaner) DeleteVPCResources(vpcID string) error {
	c.log.Info("Starting cleanup of resources in VPC: %s", vpcID)

	// Delete instances
	if err := c.deleteInstances(vpcID); err != nil {
		return fmt.Errorf("failed to delete instances: %w", err)
	}

	// Delete security groups
	if err := c.deleteSecurityGroups(vpcID); err != nil {
		return fmt.Errorf("failed to delete security groups: %w", err)
	}

	// Delete subnets
	if err := c.deleteSubnets(vpcID); err != nil {
		return fmt.Errorf("failed to delete subnets: %w", err)
	}

	// Delete route tables
	if err := c.deleteRouteTables(vpcID); err != nil {
		return fmt.Errorf("failed to delete route tables: %w", err)
	}

	// Delete internet gateways
	if err := c.deleteInternetGateways(vpcID); err != nil {
		return fmt.Errorf("failed to delete internet gateways: %w", err)
	}

	// Delete VPC with retry
	if err := c.deleteVPC(vpcID); err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}

	c.log.Info("Successfully deleted VPC %s and all associated resources", vpcID)
	return nil
}

func (c *Cleaner) deleteInstances(vpcID string) error {
	// Describe instances in the VPC
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running", "stopped", "stopping", "pending"},
			},
		},
	}

	result, err := c.ec2.DescribeInstances(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to describe instances: %w", err)
	}

	var instanceIDs []string
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceIDs = append(instanceIDs, *instance.InstanceId)
		}
	}

	if len(instanceIDs) == 0 {
		c.log.Info("No instances found to delete")
		return nil
	}

	// Terminate instances
	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	}

	_, err = c.ec2.TerminateInstances(context.Background(), terminateInput)
	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}

	c.log.Info("Terminated %d instances, waiting for termination", len(instanceIDs))

	// Wait for instances to terminate
	waiter := ec2.NewInstanceTerminatedWaiter(c.ec2)
	err = waiter.Wait(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed waiting for instances to terminate: %w", err)
	}

	return nil
}

func (c *Cleaner) deleteSecurityGroups(vpcID string) error {
	// Describe security groups
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeSecurityGroups(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to describe security groups: %w", err)
	}

	// Get default security group ID for later use
	var defaultSGID string
	var nonDefaultSGs []types.SecurityGroup
	for _, sg := range result.SecurityGroups {
		if *sg.GroupName == "default" {
			defaultSGID = *sg.GroupId
		} else {
			nonDefaultSGs = append(nonDefaultSGs, sg)
		}
	}

	// First, detach security groups from ENIs
	for _, sg := range nonDefaultSGs {
		// Find ENIs using this security group
		eniInput := &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("group-id"),
					Values: []string{*sg.GroupId},
				},
			},
		}

		eniResult, err := c.ec2.DescribeNetworkInterfaces(context.Background(), eniInput)
		if err != nil {
			c.log.Warning("Failed to describe ENIs for security group %s: %v", *sg.GroupId, err)
			continue
		}

		// Modify ENIs to use default security group
		for _, eni := range eniResult.NetworkInterfaces {
			if defaultSGID != "" {
				modifyInput := &ec2.ModifyNetworkInterfaceAttributeInput{
					NetworkInterfaceId: eni.NetworkInterfaceId,
					Groups:             []string{defaultSGID},
				}

				_, err = c.ec2.ModifyNetworkInterfaceAttribute(context.Background(), modifyInput)
				if err != nil {
					c.log.Warning("Failed to modify ENI %s: %v", *eni.NetworkInterfaceId, err)
				}
			}
		}
	}

	// Delete non-default security groups
	for _, sg := range nonDefaultSGs {
		deleteInput := &ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		}

		_, err = c.ec2.DeleteSecurityGroup(context.Background(), deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete security group %s: %v", *sg.GroupId, err)
		}
	}

	c.log.Info("Deleted %d security groups", len(nonDefaultSGs))
	return nil
}

func (c *Cleaner) deleteSubnets(vpcID string) error {
	// Describe subnets
	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeSubnets(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to describe subnets: %w", err)
	}

	// Delete each subnet
	for _, subnet := range result.Subnets {
		deleteInput := &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		}

		_, err = c.ec2.DeleteSubnet(context.Background(), deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete subnet %s: %v", *subnet.SubnetId, err)
		}
	}

	c.log.Info("Deleted %d subnets", len(result.Subnets))
	return nil
}

func (c *Cleaner) deleteRouteTables(vpcID string) error {
	// Describe route tables
	input := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeRouteTables(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to describe route tables: %w", err)
	}

	// Find main route table and non-main route tables
	var mainRouteTableID string
	var nonMainRouteTables []types.RouteTable

	for _, rt := range result.RouteTables {
		isMain := false
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = true
				mainRouteTableID = *rt.RouteTableId
				break
			}
		}
		if !isMain {
			nonMainRouteTables = append(nonMainRouteTables, rt)
		}
	}

	// Replace associations for non-main route tables
	if mainRouteTableID != "" {
		for _, rt := range nonMainRouteTables {
			for _, assoc := range rt.Associations {
				if assoc.RouteTableAssociationId != nil {
					replaceInput := &ec2.ReplaceRouteTableAssociationInput{
						AssociationId: assoc.RouteTableAssociationId,
						RouteTableId:  &mainRouteTableID,
					}

					_, err = c.ec2.ReplaceRouteTableAssociation(context.Background(), replaceInput)
					if err != nil {
						c.log.Warning("Failed to replace route table association: %v", err)
					}
				}
			}
		}
	}

	// Delete non-main route tables
	for _, rt := range nonMainRouteTables {
		deleteInput := &ec2.DeleteRouteTableInput{
			RouteTableId: rt.RouteTableId,
		}

		_, err = c.ec2.DeleteRouteTable(context.Background(), deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete route table %s: %v", *rt.RouteTableId, err)
		}
	}

	c.log.Info("Deleted %d route tables", len(nonMainRouteTables))
	return nil
}

func (c *Cleaner) deleteInternetGateways(vpcID string) error {
	// Describe internet gateways
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeInternetGateways(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to describe internet gateways: %w", err)
	}

	// Detach and delete each internet gateway
	for _, igw := range result.InternetGateways {
		// Detach from VPC
		detachInput := &ec2.DetachInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
			VpcId:             &vpcID,
		}

		_, err = c.ec2.DetachInternetGateway(context.Background(), detachInput)
		if err != nil {
			c.log.Warning("Failed to detach internet gateway %s: %v", *igw.InternetGatewayId, err)
		}

		// Delete internet gateway
		deleteInput := &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
		}

		_, err = c.ec2.DeleteInternetGateway(context.Background(), deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete internet gateway %s: %v", *igw.InternetGatewayId, err)
		}
	}

	c.log.Info("Deleted %d internet gateways", len(result.InternetGateways))
	return nil
}

func (c *Cleaner) deleteVPC(vpcID string) error {
	// Try to delete VPC with retries
	attempts := 0
	maxAttempts := 3

	for attempts < maxAttempts {
		deleteInput := &ec2.DeleteVpcInput{
			VpcId: &vpcID,
		}

		_, err := c.ec2.DeleteVpc(context.Background(), deleteInput)
		if err == nil {
			c.log.Info("Successfully deleted VPC: %s", vpcID)
			return nil
		}

		attempts++
		if attempts < maxAttempts {
			c.log.Warning("Failed to delete VPC %s (attempt %d/%d): %v. Retrying in 30 seconds...",
				vpcID, attempts, maxAttempts, err)
			time.Sleep(30 * time.Second)
		}
	}

	return fmt.Errorf("failed to delete VPC %s after %d attempts", vpcID, maxAttempts)
}
