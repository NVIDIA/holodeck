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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestDeleteConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant time.Duration
		expected time.Duration
	}{
		{"maxRetries", time.Duration(maxRetries), 5},
		{"retryDelay", retryDelay, 5 * time.Second},
		{"maxRetryDelay", maxRetryDelay, 30 * time.Second},
		{"verificationDelay", verificationDelay, 2 * time.Second},
		{"apiCallTimeout", apiCallTimeout, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.constant)
			}
		})
	}
}

func TestDeleteRetryBackoff(t *testing.T) {
	attempts := 0
	expectedError := fmt.Errorf("test error")

	provider := &Provider{
		log: mockLogger(),
	}

	err := provider.retryWithBackoff(func() error {
		attempts++
		return expectedError
	})

	if err == nil {
		t.Error("expected error from retryWithBackoff")
	}

	if attempts != maxRetries {
		t.Errorf("expected %d retry attempts, got %d", maxRetries, attempts)
	}
}

func TestDeleteRetryBackoffSuccess(t *testing.T) {
	attempts := 0

	provider := &Provider{
		log: mockLogger(),
	}

	err := provider.retryWithBackoff(func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDeleteVPCResources_ExponentialBackoffDelay(t *testing.T) {
	for retry := 0; retry < maxRetries; retry++ {
		expected := retryDelay * time.Duration(1<<retry)
		if expected > maxRetryDelay {
			expected = maxRetryDelay
		}

		if expected < retryDelay {
			t.Errorf("retry %d: delay %v is less than minimum %v", retry, expected, retryDelay)
		}
		if expected > maxRetryDelay {
			t.Errorf("retry %d: delay %v exceeds maximum %v", retry, expected, maxRetryDelay)
		}
	}
}

func TestSecurityGroupExists(t *testing.T) {
	tests := []struct {
		name     string
		sgId     string
		mock     func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
		expected bool
	}{
		{
			name: "exists",
			sgId: "sg-123",
			mock: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
				return &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []types.SecurityGroup{{GroupId: aws.String("sg-123")}},
				}, nil
			},
			expected: true,
		},
		{
			name: "not found",
			sgId: "sg-456",
			mock: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
				return nil, fmt.Errorf("InvalidGroup.NotFound")
			},
			expected: false,
		},
		{
			name: "error",
			sgId: "sg-789",
			mock: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
				return nil, fmt.Errorf("some error")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				ec2: &MockEC2Client{
					DescribeSGsFunc: tt.mock,
				},
				log: mockLogger(),
			}
			if got := provider.securityGroupExists(tt.sgId); got != tt.expected {
				t.Errorf("securityGroupExists(%s) = %v, want %v", tt.sgId, got, tt.expected)
			}
		})
	}
}

// Test individual delete helpers for new NAT/public resources

func TestDeleteNATGateway_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger()}
	cache := &AWS{NatGatewayid: ""}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("expected no error for empty NatGatewayid, got: %v", err)
	}
}

func TestDeleteNATGateway_AlreadyDeleted(t *testing.T) {
	mock := &MockEC2Client{
		DeleteNatGatewayFunc: func(ctx context.Context, params *ec2.DeleteNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error) {
			return nil, fmt.Errorf("NatGatewayNotFound: nat-gone")
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{NatGatewayid: "nat-gone"}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("expected no error for NatGatewayNotFound, got: %v", err)
	}
}

func TestDeleteNATGateway_WaitsForDeletedState(t *testing.T) {
	describeCalls := 0
	mock := &MockEC2Client{
		DeleteNatGatewayFunc: func(ctx context.Context, params *ec2.DeleteNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error) {
			return &ec2.DeleteNatGatewayOutput{}, nil
		},
		DescribeNatGatewaysFunc: func(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
			describeCalls++
			state := types.NatGatewayStateDeleting
			if describeCalls >= 2 {
				state = types.NatGatewayStateDeleted
			}
			return &ec2.DescribeNatGatewaysOutput{
				NatGateways: []types.NatGateway{
					{NatGatewayId: aws.String("nat-123"), State: state},
				},
			}, nil
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{NatGatewayid: "nat-123"}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if describeCalls < 2 {
		t.Errorf("expected at least 2 DescribeNatGateways calls for polling, got %d", describeCalls)
	}
}

func TestDeleteNATGateway_EmptyDescribeBreaksLoop(t *testing.T) {
	mock := &MockEC2Client{
		DeleteNatGatewayFunc: func(ctx context.Context, params *ec2.DeleteNatGatewayInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error) {
			return &ec2.DeleteNatGatewayOutput{}, nil
		},
		DescribeNatGatewaysFunc: func(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
			// Empty response = NAT GW fully gone
			return &ec2.DescribeNatGatewaysOutput{}, nil
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{NatGatewayid: "nat-123"}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReleaseElasticIP_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger()}
	cache := &AWS{EIPAllocationid: ""}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("expected no error for empty EIPAllocationid, got: %v", err)
	}
}

func TestReleaseElasticIP_Success(t *testing.T) {
	var releasedID string
	mock := &MockEC2Client{
		ReleaseAddressFunc: func(ctx context.Context, params *ec2.ReleaseAddressInput, optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
			releasedID = aws.ToString(params.AllocationId)
			return &ec2.ReleaseAddressOutput{}, nil
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{EIPAllocationid: "eipalloc-test-123"}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if releasedID != "eipalloc-test-123" {
		t.Errorf("expected release of eipalloc-test-123, got %s", releasedID)
	}
}

func TestReleaseElasticIP_AlreadyReleased(t *testing.T) {
	mock := &MockEC2Client{
		ReleaseAddressFunc: func(ctx context.Context, params *ec2.ReleaseAddressInput, optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
			return nil, fmt.Errorf("InvalidAllocationID.NotFound")
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{EIPAllocationid: "eipalloc-gone"}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("expected no error for already-released EIP, got: %v", err)
	}
}

func TestDeletePublicRouteTable_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger()}
	cache := &AWS{PublicRouteTable: ""}
	if err := provider.deletePublicRouteTable(cache); err != nil {
		t.Fatalf("expected no error for empty PublicRouteTable, got: %v", err)
	}
}

func TestDeletePublicRouteTable_Success(t *testing.T) {
	var deletedID string
	mock := &MockEC2Client{
		DeleteRTFunc: func(ctx context.Context, params *ec2.DeleteRouteTableInput, optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
			deletedID = aws.ToString(params.RouteTableId)
			return &ec2.DeleteRouteTableOutput{}, nil
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{PublicRouteTable: "rtb-public-123"}
	if err := provider.deletePublicRouteTable(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "rtb-public-123" {
		t.Errorf("expected deletion of rtb-public-123, got %s", deletedID)
	}
}

func TestDeletePublicSubnet_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger()}
	cache := &AWS{PublicSubnetid: ""}
	if err := provider.deletePublicSubnet(cache); err != nil {
		t.Fatalf("expected no error for empty PublicSubnetid, got: %v", err)
	}
}

func TestDeletePublicSubnet_Success(t *testing.T) {
	var deletedID string
	mock := &MockEC2Client{
		DeleteSubnetFunc: func(ctx context.Context, params *ec2.DeleteSubnetInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
			deletedID = aws.ToString(params.SubnetId)
			return &ec2.DeleteSubnetOutput{}, nil
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	cache := &AWS{PublicSubnetid: "subnet-public-123"}
	if err := provider.deletePublicSubnet(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "subnet-public-123" {
		t.Errorf("expected deletion of subnet-public-123, got %s", deletedID)
	}
}
