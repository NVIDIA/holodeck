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

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// testEnvironment returns a minimal Environment for tests that call
// updateProgressingCondition / updateDegradedCondition (which invoke DeepCopy).
func testEnvironment() *v1alpha1.Environment {
	return &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "t3.medium",
				Region: "us-east-1",
			},
		},
	}
}

func TestDeleteConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant time.Duration
		expected time.Duration
	}{
		{"maxRetries", time.Duration(maxRetries), 10},
		{"retryDelay", retryDelay, 10 * time.Second},
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

// Tests for dual security group cleanup

func TestDeleteSecurityGroup_EmptyID(t *testing.T) {
	provider := &Provider{log: mockLogger()}
	if err := provider.deleteSecurityGroup("", "worker"); err != nil {
		t.Fatalf("expected no error for empty SG ID, got: %v", err)
	}
}

func TestDeleteSecurityGroup_AlreadyDeleted(t *testing.T) {
	mock := &MockEC2Client{
		DeleteSGFunc: func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound: sg-gone")
		},
		DescribeSGsFunc: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound")
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	if err := provider.deleteSecurityGroup("sg-gone", "control-plane"); err != nil {
		t.Fatalf("expected no error for InvalidGroup.NotFound, got: %v", err)
	}
}

func TestDeleteSecurityGroup_Success(t *testing.T) {
	var deletedID string
	mock := &MockEC2Client{
		DeleteSGFunc: func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			deletedID = aws.ToString(params.GroupId)
			return &ec2.DeleteSecurityGroupOutput{}, nil
		},
		DescribeSGsFunc: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound")
		},
	}
	provider := &Provider{ec2: mock, log: mockLogger()}
	if err := provider.deleteSecurityGroup("sg-cp-123", "control-plane"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "sg-cp-123" {
		t.Errorf("expected deletion of sg-cp-123, got %s", deletedID)
	}
}

func TestDeleteSecurityGroups_DualSG_DeleteOrder(t *testing.T) {
	// Verify Worker SG is deleted before CP SG (dependency order)
	var deleteOrder []string
	mock := &MockEC2Client{
		DeleteSGFunc: func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			deleteOrder = append(deleteOrder, aws.ToString(params.GroupId))
			return &ec2.DeleteSecurityGroupOutput{}, nil
		},
		DescribeSGsFunc: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound")
		},
	}

	env := testEnvironment()
	provider := &Provider{
		ec2:         mock,
		log:         mockLogger(),
		Environment: env,
	}
	cache := &AWS{
		SecurityGroupid:       "sg-shared",
		CPSecurityGroupid:     "sg-cp",
		WorkerSecurityGroupid: "sg-worker",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deleteOrder) != 3 {
		t.Fatalf("expected 3 deletions, got %d: %v", len(deleteOrder), deleteOrder)
	}
	// Worker must come before CP, both before shared
	if deleteOrder[0] != "sg-worker" {
		t.Errorf("expected sg-worker deleted first, got %s", deleteOrder[0])
	}
	if deleteOrder[1] != "sg-cp" {
		t.Errorf("expected sg-cp deleted second, got %s", deleteOrder[1])
	}
	if deleteOrder[2] != "sg-shared" {
		t.Errorf("expected sg-shared deleted third, got %s", deleteOrder[2])
	}
}

func TestDeleteSecurityGroups_SingleNode_EmptyCPAndWorker(t *testing.T) {
	// Single-node mode: CP and Worker SG IDs are empty, only shared SG exists
	var deletedIDs []string
	mock := &MockEC2Client{
		DeleteSGFunc: func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			deletedIDs = append(deletedIDs, aws.ToString(params.GroupId))
			return &ec2.DeleteSecurityGroupOutput{}, nil
		},
		DescribeSGsFunc: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound")
		},
	}

	env := testEnvironment()
	provider := &Provider{
		ec2:         mock,
		log:         mockLogger(),
		Environment: env,
	}
	cache := &AWS{
		SecurityGroupid:       "sg-shared",
		CPSecurityGroupid:     "",
		WorkerSecurityGroupid: "",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deletedIDs) != 1 {
		t.Fatalf("expected 1 deletion (shared SG only), got %d: %v", len(deletedIDs), deletedIDs)
	}
	if deletedIDs[0] != "sg-shared" {
		t.Errorf("expected sg-shared, got %s", deletedIDs[0])
	}
}

func TestDeleteSecurityGroups_AllEmpty(t *testing.T) {
	env := testEnvironment()
	provider := &Provider{
		log:         mockLogger(),
		Environment: env,
	}
	cache := &AWS{}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("expected no error when all SG IDs are empty, got: %v", err)
	}
}

func TestDeleteSecurityGroups_SharedSameAsCP(t *testing.T) {
	// When SecurityGroupid == CPSecurityGroupid (single-node mode where both
	// point to the same SG), the shared SG should NOT be double-deleted.
	var deletedSGs []string
	mock := &MockEC2Client{
		DeleteSGFunc: func(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			deletedSGs = append(deletedSGs, aws.ToString(params.GroupId))
			return &ec2.DeleteSecurityGroupOutput{}, nil
		},
		DescribeSGsFunc: func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, fmt.Errorf("InvalidGroup.NotFound")
		},
	}

	env := testEnvironment()
	provider := &Provider{
		ec2:         mock,
		log:         mockLogger(),
		Environment: env,
	}

	cache := &AWS{
		SecurityGroupid:       "sg-same-001",
		CPSecurityGroupid:     "sg-same-001",
		WorkerSecurityGroupid: "sg-worker-002",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should delete worker and CP, but NOT double-delete the shared SG
	if len(deletedSGs) != 2 {
		t.Fatalf("expected 2 delete calls, got %d: %v", len(deletedSGs), deletedSGs)
	}
	if deletedSGs[0] != "sg-worker-002" {
		t.Errorf("first delete should be worker SG, got %s", deletedSGs[0])
	}
	if deletedSGs[1] != "sg-same-001" {
		t.Errorf("second delete should be CP SG (same as shared), got %s", deletedSGs[1])
	}
}

func TestRevokeSecurityGroupRules_RevokesIngressOnly(t *testing.T) {
	var ingressCalls []ec2.RevokeSecurityGroupIngressInput
	var egressCalls []ec2.RevokeSecurityGroupEgressInput

	mock := NewMockEC2Client()
	mock.DescribeSGsFunc = func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []types.SecurityGroup{
				{
					GroupId: aws.String("sg-worker"),
					IpPermissions: []types.IpPermission{
						{
							IpProtocol: aws.String("-1"),
							UserIdGroupPairs: []types.UserIdGroupPair{
								{GroupId: aws.String("sg-cp")},
							},
						},
					},
					IpPermissionsEgress: []types.IpPermission{
						{
							IpProtocol: aws.String("-1"),
							IpRanges: []types.IpRange{
								{CidrIp: aws.String("0.0.0.0/0")},
							},
						},
					},
				},
			},
		}, nil
	}
	mock.RevokeSGIngressFunc = func(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
		ingressCalls = append(ingressCalls, *params)
		return &ec2.RevokeSecurityGroupIngressOutput{}, nil
	}
	mock.RevokeSGEgressFunc = func(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput,
		optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
		egressCalls = append(egressCalls, *params)
		return &ec2.RevokeSecurityGroupEgressOutput{}, nil
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.revokeSecurityGroupRules("sg-worker")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules failed: %v", err)
	}

	if len(ingressCalls) != 1 {
		t.Fatalf("Expected 1 RevokeSecurityGroupIngress call, got %d", len(ingressCalls))
	}
	if *ingressCalls[0].GroupId != "sg-worker" {
		t.Errorf("Expected GroupId 'sg-worker', got %q", *ingressCalls[0].GroupId)
	}

	// Egress revocation is intentionally skipped — CI IAM user lacks
	// ec2:RevokeSecurityGroupEgress, and the default egress rule does not
	// create cross-SG dependencies that block DeleteSecurityGroup.
	if len(egressCalls) != 0 {
		t.Errorf("Expected 0 RevokeSecurityGroupEgress calls (egress skipped), got %d", len(egressCalls))
	}
}

func TestRevokeSecurityGroupRules_SkipsEmptyRules(t *testing.T) {
	var ingressCalls []ec2.RevokeSecurityGroupIngressInput
	var egressCalls []ec2.RevokeSecurityGroupEgressInput

	mock := NewMockEC2Client()
	mock.DescribeSGsFunc = func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []types.SecurityGroup{
				{
					GroupId:             aws.String("sg-empty"),
					IpPermissions:       nil,
					IpPermissionsEgress: nil,
				},
			},
		}, nil
	}
	mock.RevokeSGIngressFunc = func(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
		ingressCalls = append(ingressCalls, *params)
		return &ec2.RevokeSecurityGroupIngressOutput{}, nil
	}
	mock.RevokeSGEgressFunc = func(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput,
		optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
		egressCalls = append(egressCalls, *params)
		return &ec2.RevokeSecurityGroupEgressOutput{}, nil
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.revokeSecurityGroupRules("sg-empty")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules failed: %v", err)
	}

	if len(ingressCalls) != 0 {
		t.Errorf("Expected no RevokeSecurityGroupIngress calls for empty rules, got %d", len(ingressCalls))
	}
	if len(egressCalls) != 0 {
		t.Errorf("Expected no RevokeSecurityGroupEgress calls for empty rules, got %d", len(egressCalls))
	}
}

func TestRevokeSecurityGroupRules_SkipsEmptyID(t *testing.T) {
	mock := NewMockEC2Client()
	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.revokeSecurityGroupRules("")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules should skip empty SG ID, got: %v", err)
	}
}

func TestRevokeSecurityGroupRules_DescribeError(t *testing.T) {
	mock := NewMockEC2Client()
	mock.DescribeSGsFunc = func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
		return nil, fmt.Errorf("InvalidGroup.NotFound")
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	// NotFound is not an error — SG is already gone
	err := provider.revokeSecurityGroupRules("sg-gone")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules should handle NotFound gracefully, got: %v", err)
	}
}

func TestWaitForENIsDrained_NoENIs(t *testing.T) {
	mock := NewMockEC2Client()
	mock.DescribeNIsFunc = func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
		return &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []types.NetworkInterface{},
		}, nil
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should succeed with no ENIs, got: %v", err)
	}
}

func TestWaitForENIsDrained_SkipsEmptyVPC(t *testing.T) {
	mock := NewMockEC2Client()
	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.waitForENIsDrained("")
	if err != nil {
		t.Fatalf("waitForENIsDrained should skip empty VPC ID, got: %v", err)
	}
}

func TestWaitForENIsDrained_ENIsDrainOnSecondPoll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: poll loop sleeps 10s between calls")
	}
	callCount := 0
	mock := NewMockEC2Client()
	mock.DescribeNIsFunc = func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
		callCount++
		if callCount == 1 {
			// First call: ENI still in-use
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						NetworkInterfaceId: aws.String("eni-123"),
						Status:             types.NetworkInterfaceStatusInUse,
					},
				},
			}, nil
		}
		// Second call: all drained
		return &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []types.NetworkInterface{},
		}, nil
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should succeed after ENIs drain, got: %v", err)
	}
	if callCount < 2 {
		t.Errorf("Expected at least 2 DescribeNetworkInterfaces calls, got %d", callCount)
	}
}

func TestWaitForENIsDrained_AvailableENIsIgnored(t *testing.T) {
	mock := NewMockEC2Client()
	mock.DescribeNIsFunc = func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
		// ENI exists but is in "available" state (detached) — not blocking
		return &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []types.NetworkInterface{
				{
					NetworkInterfaceId: aws.String("eni-avail"),
					Status:             types.NetworkInterfaceStatusAvailable,
				},
			},
		}, nil
	}

	provider := &Provider{ec2: mock, log: mockLogger()}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should ignore 'available' ENIs, got: %v", err)
	}
}
