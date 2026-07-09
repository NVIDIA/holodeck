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
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"

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

// seedSG registers a security group in the store under an exact id so a delete
// path can genuinely remove it (CreateSecurityGroup would assign a fake id).
func seedSG(f *awsfake.Fake, id string) {
	f.Store.SecurityGroups[id] = &types.SecurityGroup{GroupId: aws.String(id)}
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

	var sleepCalls []time.Duration
	provider := &Provider{
		log:   mockLogger(),
		sleep: func(d time.Duration) { sleepCalls = append(sleepCalls, d) },
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

	// Verify exponential backoff delays were passed to sleep
	if len(sleepCalls) != maxRetries-1 {
		t.Fatalf("expected %d sleep calls, got %d", maxRetries-1, len(sleepCalls))
	}
	if sleepCalls[0] != retryDelay {
		t.Errorf("first sleep should be %v, got %v", retryDelay, sleepCalls[0])
	}
}

func TestDeleteRetryBackoffSuccess(t *testing.T) {
	attempts := 0

	provider := &Provider{
		log:   mockLogger(),
		sleep: noopSleep,
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
		setup    func(*awsfake.Fake) string // returns the SG id to check
		expected bool
	}{
		{
			name: "exists",
			setup: func(f *awsfake.Fake) string {
				sg, _ := f.EC2.CreateSecurityGroup(context.Background(), &ec2.CreateSecurityGroupInput{
					GroupName: aws.String("g"), VpcId: aws.String("vpc-1"),
				})
				return aws.ToString(sg.GroupId)
			},
			expected: true,
		},
		{
			name:     "not found",
			setup:    func(f *awsfake.Fake) string { return "sg-456" }, // never created -> InvalidGroup.NotFound
			expected: false,
		},
		{
			name: "error",
			setup: func(f *awsfake.Fake) string {
				f.Store.FailNext("DescribeSecurityGroups", fmt.Errorf("some error"))
				return "sg-789"
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := awsfake.New()
			sgID := tt.setup(f)
			provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
			if got := provider.securityGroupExists(sgID); got != tt.expected {
				t.Errorf("securityGroupExists(%s) = %v, want %v", sgID, got, tt.expected)
			}
		})
	}
}

// Test individual delete helpers for new NAT/public resources

func TestDeleteNATGateway_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger(), sleep: noopSleep}
	cache := &AWS{NatGatewayid: ""}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("expected no error for empty NatGatewayid, got: %v", err)
	}
}

func TestDeleteNATGateway_AlreadyDeleted(t *testing.T) {
	// An absent NAT gateway makes DeleteNatGateway return NatGatewayNotFound,
	// which deleteNATGateway treats as success.
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{NatGatewayid: "nat-gone"}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("expected no error for NatGatewayNotFound, got: %v", err)
	}
}

func TestDeleteNATGateway_WaitsForDeletedState(t *testing.T) {
	f := awsfake.New()
	nat, _ := f.EC2.CreateNatGateway(context.Background(), &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-1")})
	natID := aws.ToString(nat.NatGateway.NatGatewayId)
	// Report "deleting" on the first describe, gone on the second.
	f.Store.SeedNextNatGatewayDeleteState(1)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{NatGatewayid: natID}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := f.Store.CallsTo("DescribeNatGateways"); got < 2 {
		t.Errorf("expected at least 2 DescribeNatGateways calls for polling, got %d", got)
	}
}

func TestDeleteNATGateway_EmptyDescribeBreaksLoop(t *testing.T) {
	// Default delete removes the gateway immediately, so the first describe
	// returns empty and the wait loop breaks without error.
	f := awsfake.New()
	nat, _ := f.EC2.CreateNatGateway(context.Background(), &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-1")})
	natID := aws.ToString(nat.NatGateway.NatGatewayId)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{NatGatewayid: natID}
	if err := provider.deleteNATGateway(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReleaseElasticIP_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger(), sleep: noopSleep}
	cache := &AWS{EIPAllocationid: ""}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("expected no error for empty EIPAllocationid, got: %v", err)
	}
}

func TestReleaseElasticIP_Success(t *testing.T) {
	f := awsfake.New()
	alloc, _ := f.EC2.AllocateAddress(context.Background(), &ec2.AllocateAddressInput{})
	eipID := aws.ToString(alloc.AllocationId)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{EIPAllocationid: eipID}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	releaseCalls := f.Store.Inputs("ReleaseAddress")
	if len(releaseCalls) == 0 {
		t.Fatal("ReleaseAddress was not called")
	}
	if got := aws.ToString(releaseCalls[0].(*ec2.ReleaseAddressInput).AllocationId); got != eipID {
		t.Errorf("expected release of %q, got %q", eipID, got)
	}
	if len(f.Store.Addresses) != 0 {
		t.Errorf("expected the EIP to be released, %d address(es) remain", len(f.Store.Addresses))
	}
}

func TestReleaseElasticIP_AlreadyReleased(t *testing.T) {
	// An absent allocation makes ReleaseAddress return InvalidAllocationID.NotFound,
	// which releaseElasticIP treats as success.
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{EIPAllocationid: "eipalloc-gone"}
	if err := provider.releaseElasticIP(cache); err != nil {
		t.Fatalf("expected no error for already-released EIP, got: %v", err)
	}
}

func TestDeletePublicRouteTable_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger(), sleep: noopSleep}
	cache := &AWS{PublicRouteTable: ""}
	if err := provider.deletePublicRouteTable(cache); err != nil {
		t.Fatalf("expected no error for empty PublicRouteTable, got: %v", err)
	}
}

func TestDeletePublicRouteTable_Success(t *testing.T) {
	f := awsfake.New()
	rt, _ := f.EC2.CreateRouteTable(context.Background(), &ec2.CreateRouteTableInput{VpcId: aws.String("vpc-1")})
	rtID := aws.ToString(rt.RouteTable.RouteTableId)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{PublicRouteTable: rtID}
	if err := provider.deletePublicRouteTable(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleteCalls := f.Store.Inputs("DeleteRouteTable")
	if len(deleteCalls) == 0 {
		t.Fatal("DeleteRouteTable was not called")
	}
	if got := aws.ToString(deleteCalls[0].(*ec2.DeleteRouteTableInput).RouteTableId); got != rtID {
		t.Errorf("expected deletion of %q, got %q", rtID, got)
	}
	if _, ok := f.Store.RouteTables[rtID]; ok {
		t.Error("route table was not removed from the store")
	}
}

func TestDeletePublicSubnet_Empty(t *testing.T) {
	provider := &Provider{log: mockLogger(), sleep: noopSleep}
	cache := &AWS{PublicSubnetid: ""}
	if err := provider.deletePublicSubnet(cache); err != nil {
		t.Fatalf("expected no error for empty PublicSubnetid, got: %v", err)
	}
}

func TestDeletePublicSubnet_Success(t *testing.T) {
	f := awsfake.New()
	sn, _ := f.EC2.CreateSubnet(context.Background(), &ec2.CreateSubnetInput{VpcId: aws.String("vpc-1"), CidrBlock: aws.String("10.0.1.0/24")})
	snID := aws.ToString(sn.Subnet.SubnetId)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	cache := &AWS{PublicSubnetid: snID}
	if err := provider.deletePublicSubnet(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleteCalls := f.Store.Inputs("DeleteSubnet")
	if len(deleteCalls) == 0 {
		t.Fatal("DeleteSubnet was not called")
	}
	if got := aws.ToString(deleteCalls[0].(*ec2.DeleteSubnetInput).SubnetId); got != snID {
		t.Errorf("expected deletion of %q, got %q", snID, got)
	}
	if _, ok := f.Store.Subnets[snID]; ok {
		t.Error("subnet was not removed from the store")
	}
}

// Tests for dual security group cleanup

func TestDeleteSecurityGroup_EmptyID(t *testing.T) {
	provider := &Provider{log: mockLogger(), sleep: noopSleep}
	if err := provider.deleteSecurityGroup("", "worker"); err != nil {
		t.Fatalf("expected no error for empty SG ID, got: %v", err)
	}
}

func TestDeleteSecurityGroup_AlreadyDeleted(t *testing.T) {
	// An absent SG makes DeleteSecurityGroup return InvalidGroup.NotFound, which
	// deleteSecurityGroup treats as success.
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	if err := provider.deleteSecurityGroup("sg-gone", "control-plane"); err != nil {
		t.Fatalf("expected no error for InvalidGroup.NotFound, got: %v", err)
	}
}

func TestDeleteSecurityGroup_Success(t *testing.T) {
	f := awsfake.New()
	seedSG(f, "sg-cp-123")

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}
	if err := provider.deleteSecurityGroup("sg-cp-123", "control-plane"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleteCalls := f.Store.Inputs("DeleteSecurityGroup")
	if len(deleteCalls) == 0 {
		t.Fatal("DeleteSecurityGroup was not called")
	}
	if got := aws.ToString(deleteCalls[0].(*ec2.DeleteSecurityGroupInput).GroupId); got != "sg-cp-123" {
		t.Errorf("expected deletion of sg-cp-123, got %q", got)
	}
	if _, ok := f.Store.SecurityGroups["sg-cp-123"]; ok {
		t.Error("security group was not removed from the store")
	}
}

// deleteOrder returns the GroupIds passed to DeleteSecurityGroup, in call order.
func deleteOrder(f *awsfake.Fake) []string {
	calls := f.Store.Inputs("DeleteSecurityGroup")
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, aws.ToString(c.(*ec2.DeleteSecurityGroupInput).GroupId))
	}
	return out
}

func TestDeleteSecurityGroups_DualSG_DeleteOrder(t *testing.T) {
	// Verify Worker SG is deleted before CP SG (dependency order)
	f := awsfake.New()

	env := testEnvironment()
	provider := &Provider{
		ec2:         f.EC2,
		log:         mockLogger(),
		Environment: env,
		sleep:       noopSleep,
	}
	cache := &AWS{
		SecurityGroupid:       "sg-shared",
		CPSecurityGroupid:     "sg-cp",
		WorkerSecurityGroupid: "sg-worker",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := deleteOrder(f)
	if len(order) != 3 {
		t.Fatalf("expected 3 deletions, got %d: %v", len(order), order)
	}
	// Worker must come before CP, both before shared
	if order[0] != "sg-worker" {
		t.Errorf("expected sg-worker deleted first, got %s", order[0])
	}
	if order[1] != "sg-cp" {
		t.Errorf("expected sg-cp deleted second, got %s", order[1])
	}
	if order[2] != "sg-shared" {
		t.Errorf("expected sg-shared deleted third, got %s", order[2])
	}
}

func TestDeleteSecurityGroups_SingleNode_EmptyCPAndWorker(t *testing.T) {
	// Single-node mode: CP and Worker SG IDs are empty, only shared SG exists
	f := awsfake.New()

	env := testEnvironment()
	provider := &Provider{
		ec2:         f.EC2,
		log:         mockLogger(),
		Environment: env,
		sleep:       noopSleep,
	}
	cache := &AWS{
		SecurityGroupid:       "sg-shared",
		CPSecurityGroupid:     "",
		WorkerSecurityGroupid: "",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := deleteOrder(f)
	if len(order) != 1 {
		t.Fatalf("expected 1 deletion (shared SG only), got %d: %v", len(order), order)
	}
	if order[0] != "sg-shared" {
		t.Errorf("expected sg-shared, got %s", order[0])
	}
}

func TestDeleteSecurityGroups_AllEmpty(t *testing.T) {
	env := testEnvironment()
	provider := &Provider{
		log:         mockLogger(),
		Environment: env,
		sleep:       noopSleep,
	}
	cache := &AWS{}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("expected no error when all SG IDs are empty, got: %v", err)
	}
}

func TestDeleteSecurityGroups_SharedSameAsCP(t *testing.T) {
	// When SecurityGroupid == CPSecurityGroupid (single-node mode where both
	// point to the same SG), the shared SG should NOT be double-deleted.
	f := awsfake.New()

	env := testEnvironment()
	provider := &Provider{
		ec2:         f.EC2,
		log:         mockLogger(),
		Environment: env,
		sleep:       noopSleep,
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
	order := deleteOrder(f)
	if len(order) != 2 {
		t.Fatalf("expected 2 delete calls, got %d: %v", len(order), order)
	}
	if order[0] != "sg-worker-002" {
		t.Errorf("first delete should be worker SG, got %s", order[0])
	}
	if order[1] != "sg-same-001" {
		t.Errorf("second delete should be CP SG (same as shared), got %s", order[1])
	}
}

// TestDeleteSecurityGroups_DualSG_RevokesCrossReferencePair pins the
// bug caught by delete.go:272-279: deleteSecurityGroups must revoke each
// cross-referencing security group's ingress rules before any
// DeleteSecurityGroup call is issued for the CP/worker pair. This fake
// tolerates DeleteSecurityGroup even while cross-SG ingress rules still
// reference the deleted group, so a silently dropped revoke step passes here
// but fails against real AWS with DependencyViolation (worker SG references
// CP SG, and vice versa, per createControlPlaneSecurityGroup/
// createWorkerSecurityGroup). awsfake.Store's recorder (Inputs/CallsTo) only
// tracks per-method call order/counts, not a cross-method timeline, so the
// strongest available pin (without modifying awsfake) is: both revokes
// actually fire, targeting exactly the CP/worker pair.
func TestDeleteSecurityGroups_DualSG_RevokesCrossReferencePair(t *testing.T) {
	f := awsfake.New()

	// Seed CP and Worker SGs with mutual ingress cross-references, mirroring
	// createControlPlaneSecurityGroup/createWorkerSecurityGroup's UserIdGroupPairs.
	f.Store.SecurityGroups["sg-cp"] = &types.SecurityGroup{
		GroupId: aws.String("sg-cp"),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol:       aws.String("tcp"),
				FromPort:         aws.Int32(6443),
				ToPort:           aws.Int32(6443),
				UserIdGroupPairs: []types.UserIdGroupPair{{GroupId: aws.String("sg-worker")}},
			},
		},
	}
	f.Store.SecurityGroups["sg-worker"] = &types.SecurityGroup{
		GroupId: aws.String("sg-worker"),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol:       aws.String("tcp"),
				FromPort:         aws.Int32(10250),
				ToPort:           aws.Int32(10250),
				UserIdGroupPairs: []types.UserIdGroupPair{{GroupId: aws.String("sg-cp")}},
			},
		},
	}

	env := testEnvironment()
	provider := &Provider{
		ec2:         f.EC2,
		log:         mockLogger(),
		Environment: env,
		sleep:       noopSleep,
	}
	cache := &AWS{
		SecurityGroupid:       "sg-cp",
		CPSecurityGroupid:     "sg-cp",
		WorkerSecurityGroupid: "sg-worker",
	}

	if err := provider.deleteSecurityGroups(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	revokeCalls := f.Store.Inputs("RevokeSecurityGroupIngress")
	if len(revokeCalls) != 2 {
		t.Fatalf("expected 2 RevokeSecurityGroupIngress calls (CP + worker), got %d: %v", len(revokeCalls), revokeCalls)
	}
	revoked := map[string]bool{}
	for _, c := range revokeCalls {
		revoked[aws.ToString(c.(*ec2.RevokeSecurityGroupIngressInput).GroupId)] = true
	}
	if !revoked["sg-cp"] || !revoked["sg-worker"] {
		t.Errorf("expected RevokeSecurityGroupIngress for both sg-cp and sg-worker, got %v", revoked)
	}

	// Also confirm both groups were subsequently deleted (sg-cp doubles as
	// the shared SG here, so only 2 deletes: worker, then cp).
	order := deleteOrder(f)
	if len(order) != 2 {
		t.Fatalf("expected 2 DeleteSecurityGroup calls, got %d: %v", len(order), order)
	}
}

func TestRevokeSecurityGroupRules_RevokesIngressOnly(t *testing.T) {
	f := awsfake.New()
	// Seed a worker SG with both an ingress rule and an egress rule.
	f.Store.SecurityGroups["sg-worker"] = &types.SecurityGroup{
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
	}

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.revokeSecurityGroupRules("sg-worker")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules failed: %v", err)
	}

	ingressCalls := f.Store.Inputs("RevokeSecurityGroupIngress")
	if len(ingressCalls) != 1 {
		t.Fatalf("Expected 1 RevokeSecurityGroupIngress call, got %d", len(ingressCalls))
	}
	if got := aws.ToString(ingressCalls[0].(*ec2.RevokeSecurityGroupIngressInput).GroupId); got != "sg-worker" {
		t.Errorf("Expected GroupId 'sg-worker', got %q", got)
	}

	// Egress revocation is intentionally skipped — CI IAM user lacks
	// ec2:RevokeSecurityGroupEgress, and the default egress rule does not
	// create cross-SG dependencies that block DeleteSecurityGroup.
	if got := f.Store.CallsTo("RevokeSecurityGroupEgress"); got != 0 {
		t.Errorf("Expected 0 RevokeSecurityGroupEgress calls (egress skipped), got %d", got)
	}
}

func TestRevokeSecurityGroupRules_SkipsEmptyRules(t *testing.T) {
	f := awsfake.New()
	// A security group with no ingress/egress rules.
	f.Store.SecurityGroups["sg-empty"] = &types.SecurityGroup{
		GroupId:             aws.String("sg-empty"),
		IpPermissions:       nil,
		IpPermissionsEgress: nil,
	}

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.revokeSecurityGroupRules("sg-empty")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules failed: %v", err)
	}

	if got := f.Store.CallsTo("RevokeSecurityGroupIngress"); got != 0 {
		t.Errorf("Expected no RevokeSecurityGroupIngress calls for empty rules, got %d", got)
	}
	if got := f.Store.CallsTo("RevokeSecurityGroupEgress"); got != 0 {
		t.Errorf("Expected no RevokeSecurityGroupEgress calls for empty rules, got %d", got)
	}
}

func TestRevokeSecurityGroupRules_SkipsEmptyID(t *testing.T) {
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.revokeSecurityGroupRules("")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules should skip empty SG ID, got: %v", err)
	}
}

func TestRevokeSecurityGroupRules_DescribeError(t *testing.T) {
	// An absent SG makes DescribeSecurityGroups return InvalidGroup.NotFound;
	// revokeSecurityGroupRules treats a gone SG as nothing to revoke.
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.revokeSecurityGroupRules("sg-gone")
	if err != nil {
		t.Fatalf("revokeSecurityGroupRules should handle NotFound gracefully, got: %v", err)
	}
}

func TestWaitForENIsDrained_NoENIs(t *testing.T) {
	// No interfaces in the VPC — DescribeNetworkInterfaces returns empty.
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should succeed with no ENIs, got: %v", err)
	}
}

func TestWaitForENIsDrained_SkipsEmptyVPC(t *testing.T) {
	f := awsfake.New()
	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.waitForENIsDrained("")
	if err != nil {
		t.Fatalf("waitForENIsDrained should skip empty VPC ID, got: %v", err)
	}
}

func TestWaitForENIsDrained_ENIsDrainOnSecondPoll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: poll loop sleeps 10s between calls")
	}
	f := awsfake.New()
	// One in-use ENI that blocks the first poll, then drains.
	f.Store.SeedDrainingENI("vpc-123", 1)

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should succeed after ENIs drain, got: %v", err)
	}
	if got := f.Store.CallsTo("DescribeNetworkInterfaces"); got < 2 {
		t.Errorf("Expected at least 2 DescribeNetworkInterfaces calls, got %d", got)
	}
}

func TestWaitForENIsDrained_AvailableENIsIgnored(t *testing.T) {
	// An ENI exists but is in "available" state (detached) — not blocking.
	f := awsfake.New()
	f.Store.NetworkInterfaces["eni-avail"] = &types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-avail"),
		VpcId:              aws.String("vpc-123"),
		Status:             types.NetworkInterfaceStatusAvailable,
	}

	provider := &Provider{ec2: f.EC2, log: mockLogger(), sleep: noopSleep}

	err := provider.waitForENIsDrained("vpc-123")
	if err != nil {
		t.Fatalf("waitForENIsDrained should ignore 'available' ENIs, got: %v", err)
	}
}
