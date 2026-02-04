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
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/logger"
)

// Validation patterns for GitHub API parameters
var (
	repoPattern  = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)
	runIDPattern = regexp.MustCompile(`^\d+$`)
)

// Default timeouts for cleanup operations
const (
	// DefaultOperationTimeout is the default timeout for individual AWS API operations
	DefaultOperationTimeout = 30 * time.Second
	// DefaultInstanceTerminationTimeout is the timeout for waiting for instances to terminate
	DefaultInstanceTerminationTimeout = 10 * time.Minute
	// DefaultVPCDeleteRetryDelay is the delay between VPC deletion retries
	DefaultVPCDeleteRetryDelay = 30 * time.Second
)

// safeString safely dereferences a string pointer, returning "<nil>" if the pointer is nil
func safeString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// Cleaner handles cleanup of AWS resources
type Cleaner struct {
	ec2 internalaws.EC2Client
	log *logger.FunLogger
}

// CleanerOption is a functional option for configuring the Cleaner.
type CleanerOption func(*Cleaner)

// WithEC2Client sets a custom EC2 client for the Cleaner.
// This is primarily used for testing to inject mock clients.
func WithEC2Client(client internalaws.EC2Client) CleanerOption {
	return func(c *Cleaner) {
		c.ec2 = client
	}
}

// New creates a new AWS resource cleaner.
// Optional functional options can be provided to customize the cleaner,
// such as injecting a mock EC2 client for testing.
func New(log *logger.FunLogger, region string,
	opts ...CleanerOption) (*Cleaner, error) {
	c := &Cleaner{
		log: log,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(c)
	}

	// If no EC2 client was injected, create the real one
	if c.ec2 == nil {
		// Use Background here because New is a top-level initializer
		// without caller-provided context.
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		c.ec2 = ec2.NewFromConfig(cfg)
	}

	return c, nil
}

// GetTagValue retrieves a tag value for a given VPC.
// The provided context controls cancellation and timeout for the AWS API call.
func (c *Cleaner) GetTagValue(ctx context.Context, vpcID, key string) (string, error) {
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

	result, err := c.ec2.DescribeTags(ctx, input)
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

// CheckGitHubJobsCompleted checks if all GitHub jobs are completed.
// The provided context controls cancellation and timeout for the HTTP request.
func (c *Cleaner) CheckGitHubJobsCompleted(ctx context.Context, repository, runID, token string) (bool, error) {
	// Validate input parameters to prevent URL injection
	if !repoPattern.MatchString(repository) {
		return false, fmt.Errorf("invalid repository format: %s", repository)
	}
	if !runIDPattern.MatchString(runID) {
		return false, fmt.Errorf("invalid runID format: %s", runID)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%s/jobs", repository, runID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "holodeck-cleanup/1.0")

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

// CleanupVPC checks job status and deletes VPC resources if jobs are completed.
// The provided context controls cancellation and timeout for all operations.
func (c *Cleaner) CleanupVPC(ctx context.Context, vpcID string) error {
	// Get GitHub-related tags
	repository, err := c.GetTagValue(ctx, vpcID, "GitHubRepository")
	if err != nil {
		return fmt.Errorf("failed to get GitHubRepository tag: %w", err)
	}

	runID, err := c.GetTagValue(ctx, vpcID, "GitHubRunId")
	if err != nil {
		return fmt.Errorf("failed to get GitHubRunId tag: %w", err)
	}

	// Get GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		c.log.Warning("GITHUB_TOKEN not set, skipping job status check")
	} else if repository != "" && runID != "" {
		// Check if jobs are completed
		completed, err := c.CheckGitHubJobsCompleted(ctx, repository, runID, token)
		if err != nil {
			c.log.Warning("Failed to check GitHub job status: %v", err)
		} else if !completed {
			return fmt.Errorf("github jobs are still running for vpc %s", vpcID)
		}
	}

	c.log.Info("All jobs completed or no job information found, proceeding with cleanup of VPC %s", vpcID)
	return c.DeleteVPCResources(ctx, vpcID)
}

// DeleteVPCResources deletes all resources associated with a VPC.
// The provided context controls cancellation and timeout for all operations.
// If the context is cancelled, cleanup will stop and return an error.
func (c *Cleaner) DeleteVPCResources(ctx context.Context, vpcID string) error {
	c.log.Info("Starting cleanup of resources in VPC: %s", vpcID)

	// Check for context cancellation before each step
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled: %w", err)
	}

	// Delete instances
	if err := c.deleteInstances(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete instances: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled after instance deletion: %w", err)
	}

	// Delete security groups
	if err := c.deleteSecurityGroups(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete security groups: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled after security group deletion: %w", err)
	}

	// Delete subnets
	if err := c.deleteSubnets(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete subnets: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled after subnet deletion: %w", err)
	}

	// Delete route tables
	if err := c.deleteRouteTables(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete route tables: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled after route table deletion: %w", err)
	}

	// Delete internet gateways
	if err := c.deleteInternetGateways(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete internet gateways: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup cancelled after internet gateway deletion: %w", err)
	}

	// Delete VPC with retry
	if err := c.deleteVPC(ctx, vpcID); err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}

	c.log.Info("Successfully deleted VPC %s and all associated resources", vpcID)
	return nil
}

func (c *Cleaner) deleteInstances(ctx context.Context, vpcID string) error {
	// Describe instances in the VPC
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running", "stopped", "stopping", "pending", "shutting-down"},
			},
		},
	}

	result, err := c.ec2.DescribeInstances(ctx, input)
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

	_, err = c.ec2.TerminateInstances(ctx, terminateInput)
	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}

	c.log.Info("Terminated %d instances, waiting for termination", len(instanceIDs))

	// Wait for instances to terminate with timeout
	waiter := ec2.NewInstanceTerminatedWaiter(c.ec2)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}, DefaultInstanceTerminationTimeout)
	if err != nil {
		return fmt.Errorf("failed waiting for instances to terminate: %w", err)
	}

	return nil
}

func (c *Cleaner) deleteSecurityGroups(ctx context.Context, vpcID string) error {
	// Describe security groups
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe security groups: %w", err)
	}

	// Get default security group ID for later use
	var defaultSGID string
	var nonDefaultSGs []types.SecurityGroup
	for _, sg := range result.SecurityGroups {
		if sg.GroupName != nil && *sg.GroupName == "default" {
			if sg.GroupId != nil {
				defaultSGID = *sg.GroupId
			}
		} else {
			nonDefaultSGs = append(nonDefaultSGs, sg)
		}
	}

	// First, detach security groups from ENIs
	for _, sg := range nonDefaultSGs {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during security group cleanup: %w", err)
		}

		// Find ENIs using this security group
		eniInput := &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("group-id"),
					Values: []string{*sg.GroupId},
				},
			},
		}

		eniResult, err := c.ec2.DescribeNetworkInterfaces(ctx, eniInput)
		if err != nil {
			c.log.Warning("Failed to describe ENIs for security group %s: %v", safeString(sg.GroupId), err)
			continue
		}

		// Modify ENIs to use default security group
		for _, eni := range eniResult.NetworkInterfaces {
			if defaultSGID != "" {
				modifyInput := &ec2.ModifyNetworkInterfaceAttributeInput{
					NetworkInterfaceId: eni.NetworkInterfaceId,
					Groups:             []string{defaultSGID},
				}

				_, err = c.ec2.ModifyNetworkInterfaceAttribute(ctx, modifyInput)
				if err != nil {
					c.log.Warning("Failed to modify ENI %s: %v", safeString(eni.NetworkInterfaceId), err)
				}
			}
		}
	}

	// Delete non-default security groups
	for _, sg := range nonDefaultSGs {
		deleteInput := &ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		}

		_, err = c.ec2.DeleteSecurityGroup(ctx, deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete security group %s: %v", safeString(sg.GroupId), err)
		}
	}

	c.log.Info("Deleted %d security groups", len(nonDefaultSGs))
	return nil
}

func (c *Cleaner) deleteSubnets(ctx context.Context, vpcID string) error {
	// Describe subnets
	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeSubnets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe subnets: %w", err)
	}

	// Delete each subnet
	for _, subnet := range result.Subnets {
		deleteInput := &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		}

		_, err = c.ec2.DeleteSubnet(ctx, deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete subnet %s: %v", safeString(subnet.SubnetId), err)
		}
	}

	c.log.Info("Deleted %d subnets", len(result.Subnets))
	return nil
}

func (c *Cleaner) deleteRouteTables(ctx context.Context, vpcID string) error {
	// Describe route tables
	input := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeRouteTables(ctx, input)
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
				if rt.RouteTableId != nil {
					mainRouteTableID = *rt.RouteTableId
				}
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

					_, err = c.ec2.ReplaceRouteTableAssociation(ctx, replaceInput)
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

		_, err = c.ec2.DeleteRouteTable(ctx, deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete route table %s: %v", safeString(rt.RouteTableId), err)
		}
	}

	c.log.Info("Deleted %d route tables", len(nonMainRouteTables))
	return nil
}

func (c *Cleaner) deleteInternetGateways(ctx context.Context, vpcID string) error {
	// Describe internet gateways
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.ec2.DescribeInternetGateways(ctx, input)
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

		_, err = c.ec2.DetachInternetGateway(ctx, detachInput)
		if err != nil {
			c.log.Warning("Failed to detach internet gateway %s: %v", safeString(igw.InternetGatewayId), err)
		}

		// Delete internet gateway
		deleteInput := &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
		}

		_, err = c.ec2.DeleteInternetGateway(ctx, deleteInput)
		if err != nil {
			c.log.Warning("Failed to delete internet gateway %s: %v", safeString(igw.InternetGatewayId), err)
		}
	}

	c.log.Info("Deleted %d internet gateways", len(result.InternetGateways))
	return nil
}

func (c *Cleaner) deleteVPC(ctx context.Context, vpcID string) error {
	// Try to delete VPC with retries
	attempts := 0
	maxAttempts := 3

	for attempts < maxAttempts {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("VPC deletion cancelled: %w", err)
		}

		deleteInput := &ec2.DeleteVpcInput{
			VpcId: &vpcID,
		}

		_, err := c.ec2.DeleteVpc(ctx, deleteInput)
		if err == nil {
			c.log.Info("Successfully deleted VPC: %s", vpcID)
			return nil
		}

		attempts++
		if attempts < maxAttempts {
			c.log.Warning("Failed to delete VPC %s (attempt %d/%d): %v. Retrying in %v...",
				vpcID, attempts, maxAttempts, err, DefaultVPCDeleteRetryDelay)

			// Use a timer that respects context cancellation
			select {
			case <-ctx.Done():
				return fmt.Errorf("VPC deletion cancelled during retry wait: %w", ctx.Err())
			case <-time.After(DefaultVPCDeleteRetryDelay):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed to delete VPC %s after %d attempts", vpcID, maxAttempts)
}
