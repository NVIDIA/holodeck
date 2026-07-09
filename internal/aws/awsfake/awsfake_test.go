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

package awsfake

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

var ctx = context.Background()

// TestFakeSatisfiesInterfaces is the compile-time contract T2 relies on: the
// three fake clients must be assignable to the internal aws interfaces so they
// can be injected through WithEC2Client/WithSSMClient/WithELBv2Client.
func TestFakeSatisfiesInterfaces(t *testing.T) {
	f := New()
	var _ internalaws.EC2Client = f.EC2
	var _ internalaws.ELBv2Client = f.ELBv2
	var _ internalaws.SSMClient = f.SSM
	if f.Store == nil {
		t.Fatal("New() must expose a non-nil Store")
	}
}

// TestVPCRoundTripAndNotFoundCode protects the provider's idempotent-delete
// branch (delete.go:690) which matches on the exact "InvalidVpcID.NotFound"
// substring, and the create/describe/count contract.
func TestVPCRoundTripAndNotFoundCode(t *testing.T) {
	f := New()

	out, err := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")})
	if err != nil {
		t.Fatalf("CreateVpc: %v", err)
	}
	if out.Vpc == nil || aws.ToString(out.Vpc.VpcId) == "" {
		t.Fatal("CreateVpc must return a non-nil Vpc with a VpcId")
	}
	if out.Vpc.State != ec2types.VpcStateAvailable {
		t.Fatalf("CreateVpc state = %q, want available", out.Vpc.State)
	}
	id := aws.ToString(out.Vpc.VpcId)

	if got := f.Store.ResourceCounts()["vpcs"]; got != 1 {
		t.Fatalf("vpcs count after create = %d, want 1", got)
	}

	// DescribeVpcs by id must succeed while it exists (provider.vpcExists relies
	// on err==nil meaning "still there").
	if _, err := f.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{id}}); err != nil {
		t.Fatalf("DescribeVpcs(existing) unexpected error: %v", err)
	}

	if _, err := f.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(id)}); err != nil {
		t.Fatalf("DeleteVpc: %v", err)
	}
	if got := f.Store.ResourceCounts()["vpcs"]; got != 0 {
		t.Fatalf("vpcs count after delete = %d, want 0", got)
	}

	// After deletion, DescribeVpcs by id must error so vpcExists() returns false.
	if _, err := f.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{id}}); err == nil {
		t.Fatal("DescribeVpcs(deleted) must return an error")
	}

	// Deleting an absent VPC must carry the exact retry-branch code substring.
	_, err = f.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String("vpc-does-not-exist")})
	if err == nil || !strings.Contains(err.Error(), "InvalidVpcID.NotFound") {
		t.Fatalf("DeleteVpc(absent) err = %v, want substring InvalidVpcID.NotFound", err)
	}
}

// TestDeleteAbsentCodesMatchRetryBranches asserts every not-found code the
// provider's idempotent delete/detach branches string-match on. If any code
// drifts, the provider would treat a real failure as "already gone".
func TestDeleteAbsentCodesMatchRetryBranches(t *testing.T) {
	f := New()

	tests := []struct {
		name string
		call func() error
		code string
	}{
		{"DeleteVpc", func() error {
			_, err := f.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String("vpc-x")})
			return err
		}, "InvalidVpcID.NotFound"},
		{"DeleteSubnet", func() error {
			_, err := f.EC2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: aws.String("subnet-x")})
			return err
		}, "InvalidSubnetID.NotFound"},
		{"DeleteRouteTable", func() error {
			_, err := f.EC2.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{RouteTableId: aws.String("rtb-x")})
			return err
		}, "InvalidRouteTableID.NotFound"},
		{"DeleteSecurityGroup", func() error {
			_, err := f.EC2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String("sg-x")})
			return err
		}, "InvalidGroup.NotFound"},
		{"DetachInternetGateway", func() error {
			_, err := f.EC2.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: aws.String("igw-x"), VpcId: aws.String("vpc-x")})
			return err
		}, "InvalidInternetGatewayID.NotFound"},
		{"DeleteInternetGateway", func() error {
			_, err := f.EC2.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{InternetGatewayId: aws.String("igw-x")})
			return err
		}, "InvalidInternetGatewayID.NotFound"},
		{"TerminateInstances", func() error {
			_, err := f.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{"i-x"}})
			return err
		}, "InvalidInstanceID.NotFound"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("%s(absent) err = %v, want substring %q", tc.name, err, tc.code)
			}
		})
	}
}

// TestDetachInternetGatewayAlreadyDetached exercises the second idempotency
// code the provider's isAlreadyDetachedError matches: an IGW that exists but is
// already detached returns Gateway.NotAttached (distinct from the absent-IGW
// InvalidInternetGatewayID.NotFound case).
func TestDetachInternetGatewayAlreadyDetached(t *testing.T) {
	f := New()
	vpc, _ := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")})
	igw, _ := f.EC2.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	igwID := igw.InternetGateway.InternetGatewayId

	if _, err := f.EC2.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igwID, VpcId: vpc.Vpc.VpcId}); err != nil {
		t.Fatalf("AttachInternetGateway: %v", err)
	}
	// First detach succeeds.
	if _, err := f.EC2.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
		InternetGatewayId: igwID, VpcId: vpc.Vpc.VpcId}); err != nil {
		t.Fatalf("first DetachInternetGateway: %v", err)
	}
	// Second detach on the now-detached (but still existing) IGW.
	_, err := f.EC2.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
		InternetGatewayId: igwID, VpcId: vpc.Vpc.VpcId})
	if err == nil || !strings.Contains(err.Error(), "Gateway.NotAttached") {
		t.Fatalf("re-detach err = %v, want substring Gateway.NotAttached", err)
	}
}

// TestInstanceLifecycle covers the running-waiter contract (immediate running
// state + non-nil PublicDnsName which create.go:531 dereferences), the
// terminated-waiter contract, and ENI drain on termination (delete.go
// waitForENIsDrained).
func TestInstanceLifecycle(t *testing.T) {
	f := New()

	vpc, _ := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")})
	vpcID := aws.ToString(vpc.Vpc.VpcId)
	sub, _ := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{VpcId: vpc.Vpc.VpcId, CidrBlock: aws.String("10.0.0.0/24")})
	subID := aws.ToString(sub.Subnet.SubnetId)

	run, err := f.EC2.RunInstances(ctx, &ec2.RunInstancesInput{
		MinCount: aws.Int32(1), MaxCount: aws.Int32(1),
		NetworkInterfaces: []ec2types.InstanceNetworkInterfaceSpecification{
			{SubnetId: aws.String(subID), DeviceIndex: aws.Int32(0)},
		},
	})
	if err != nil {
		t.Fatalf("RunInstances: %v", err)
	}
	if len(run.Instances) != 1 {
		t.Fatalf("RunInstances returned %d instances, want 1", len(run.Instances))
	}
	inst := run.Instances[0]
	instID := aws.ToString(inst.InstanceId)
	if instID == "" {
		t.Fatal("instance must have an InstanceId")
	}
	if inst.State == nil || inst.State.Name != ec2types.InstanceStateNameRunning {
		t.Fatalf("instance state = %v, want running", inst.State)
	}
	// create.go:531 dereferences PublicDnsName without a nil check.
	if inst.PublicDnsName == nil {
		t.Fatal("instance PublicDnsName must be non-nil (create.go dereferences it)")
	}
	// cluster.go:748-750 read these.
	if aws.ToString(inst.PublicIpAddress) == "" || aws.ToString(inst.PrivateIpAddress) == "" {
		t.Fatal("instance must have Public/Private IP addresses")
	}
	if len(inst.NetworkInterfaces) == 0 || aws.ToString(inst.NetworkInterfaces[0].NetworkInterfaceId) == "" {
		t.Fatal("instance must have a NetworkInterface with an id")
	}
	if aws.ToString(inst.SubnetId) != subID {
		t.Fatalf("instance SubnetId = %q, want %q", aws.ToString(inst.SubnetId), subID)
	}
	if got := f.Store.ResourceCounts()["instances"]; got != 1 {
		t.Fatalf("instances count = %d, want 1", got)
	}

	// DescribeInstances must report running for the waiter.
	desc, err := f.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instID}})
	if err != nil {
		t.Fatalf("DescribeInstances: %v", err)
	}
	if len(desc.Reservations) != 1 || len(desc.Reservations[0].Instances) != 1 {
		t.Fatalf("DescribeInstances grouping wrong: %+v", desc.Reservations)
	}
	if desc.Reservations[0].Instances[0].State.Name != ec2types.InstanceStateNameRunning {
		t.Fatal("DescribeInstances must report running before termination")
	}

	// ENIs present and in-use before termination (block SG deletion in reality).
	eni, _ := f.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if len(eni.NetworkInterfaces) != 1 {
		t.Fatalf("expected 1 ENI in vpc before terminate, got %d", len(eni.NetworkInterfaces))
	}

	// Terminate: state -> terminated, ENIs drained.
	if _, err := f.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{instID}}); err != nil {
		t.Fatalf("TerminateInstances: %v", err)
	}
	descT, err := f.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instID}})
	if err != nil {
		t.Fatalf("DescribeInstances(terminated): %v", err)
	}
	if descT.Reservations[0].Instances[0].State.Name != ec2types.InstanceStateNameTerminated {
		t.Fatal("terminated instance must report terminated state for the waiter")
	}
	if got := f.Store.ResourceCounts()["instances"]; got != 0 {
		t.Fatalf("live instances after terminate = %d, want 0", got)
	}
	eniAfter, _ := f.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if len(eniAfter.NetworkInterfaces) != 0 {
		t.Fatalf("expected ENIs drained after terminate, got %d", len(eniAfter.NetworkInterfaces))
	}
}

// TestFailNextFiresOnceThenClears protects the error-injection contract the
// migrated specs and T2's rollback table depend on.
func TestFailNextFiresOnceThenClears(t *testing.T) {
	f := New()
	injected := fmt.Errorf("injected boom")
	f.Store.FailNext("CreateVpc", injected)

	if _, err := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")}); err == nil {
		t.Fatal("first CreateVpc after FailNext must fail")
	}
	if _, err := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")}); err != nil {
		t.Fatalf("second CreateVpc must succeed (one-shot), got %v", err)
	}
	if got := f.Store.CallsTo("CreateVpc"); got != 2 {
		t.Fatalf("CallsTo(CreateVpc) = %d, want 2", got)
	}
}

// TestFailNextIsFIFOPerMethod verifies queued failures fire in order.
func TestFailNextIsFIFOPerMethod(t *testing.T) {
	f := New()
	f.Store.FailNext("CreateSubnet", fmt.Errorf("first"))
	f.Store.FailNext("CreateSubnet", fmt.Errorf("second"))

	_, e1 := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{})
	_, e2 := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{})
	_, e3 := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{})
	if e1 == nil || e1.Error() != "first" {
		t.Fatalf("first injected err = %v, want 'first'", e1)
	}
	if e2 == nil || e2.Error() != "second" {
		t.Fatalf("second injected err = %v, want 'second'", e2)
	}
	if e3 != nil {
		t.Fatalf("third CreateSubnet must succeed after queue drains, got %v", e3)
	}
}

// TestEmptyReflectsLiveResources is the teardown-completeness signal T2 asserts
// after Delete.
func TestEmptyReflectsLiveResources(t *testing.T) {
	f := New()
	if !f.Store.Empty() {
		t.Fatalf("New() store must be Empty (seed data excluded); counts=%v", f.Store.ResourceCounts())
	}
	vpc, _ := f.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: aws.String("10.0.0.0/16")})
	if f.Store.Empty() {
		t.Fatal("store must not be Empty after CreateVpc")
	}
	if _, err := f.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: vpc.Vpc.VpcId}); err != nil {
		t.Fatalf("DeleteVpc: %v", err)
	}
	if !f.Store.Empty() {
		t.Fatalf("store must be Empty after deleting the only resource; counts=%v", f.Store.ResourceCounts())
	}
}

// TestPaginationSinglePage guards the provider's paginated loops (image.go,
// checkInstanceTypes) which terminate only when NextToken is nil.
func TestPaginationSinglePage(t *testing.T) {
	f := New()
	img, err := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{})
	if err != nil {
		t.Fatalf("DescribeImages: %v", err)
	}
	if img.NextToken != nil {
		t.Fatal("DescribeImages must be single-page (NextToken nil)")
	}
	it, err := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
	if err != nil {
		t.Fatalf("DescribeInstanceTypes: %v", err)
	}
	if it.NextToken != nil {
		t.Fatal("DescribeInstanceTypes must be single-page (NextToken nil)")
	}
}

// TestDescribeInstanceTypesArchInference protects arm64 auto-selection
// (inferArchFromInstanceType): g5g* is arm64-only, t3* is x86_64.
func TestDescribeInstanceTypesArchInference(t *testing.T) {
	f := New()

	arm, err := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{"g5g.2xlarge"},
	})
	if err != nil {
		t.Fatalf("DescribeInstanceTypes(g5g): %v", err)
	}
	if len(arm.InstanceTypes) != 1 {
		t.Fatalf("expected echo of 1 instance type, got %d", len(arm.InstanceTypes))
	}
	if !hasArch(arm.InstanceTypes[0], ec2types.ArchitectureTypeArm64) {
		t.Fatalf("g5g.2xlarge must report arm64, got %v", arm.InstanceTypes[0].ProcessorInfo.SupportedArchitectures)
	}

	x86, _ := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{"t3.medium"},
	})
	if !hasArch(x86.InstanceTypes[0], ec2types.ArchitectureTypeX8664) {
		t.Fatalf("t3.medium must report x86_64, got %v", x86.InstanceTypes[0].ProcessorInfo.SupportedArchitectures)
	}
}

// TestDescribeInstanceTypesCatalog covers the no-filter path used by
// checkInstanceTypes: the seeded catalog must contain the types the repo's
// configs use so pre-flight validation passes against the fake.
func TestDescribeInstanceTypesCatalog(t *testing.T) {
	f := New()
	out, err := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
	if err != nil {
		t.Fatalf("DescribeInstanceTypes(catalog): %v", err)
	}
	got := map[string]bool{}
	for _, it := range out.InstanceTypes {
		got[string(it.InstanceType)] = true
	}
	for _, want := range []string{"t3.medium", "t3.large", "g4dn.xlarge", "m5.xlarge", "g5g.xlarge"} {
		if !got[want] {
			t.Fatalf("catalog missing %q; checkInstanceTypes would fail for that config", want)
		}
	}
}

// TestSeededImagesLatestSelectable seeds two Ubuntu images with distinct
// CreationDate so setAMI's latest-selection is exercised.
func TestSeededImagesLatestSelectable(t *testing.T) {
	f := New()
	out, err := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{})
	if err != nil {
		t.Fatalf("DescribeImages: %v", err)
	}
	dates := map[string]bool{}
	ubuntu := 0
	for _, img := range out.Images {
		if img.CreationDate == nil {
			t.Fatal("seeded image missing CreationDate")
		}
		dates[aws.ToString(img.CreationDate)] = true
		if strings.Contains(aws.ToString(img.Name), "ubuntu") {
			ubuntu++
		}
	}
	if ubuntu < 2 {
		t.Fatalf("expected >=2 seeded ubuntu images, got %d", ubuntu)
	}
	if len(dates) < 2 {
		t.Fatal("seeded images must have distinct CreationDate values")
	}
}

// TestDescribeImagesByID backs describeImageArch/describeImageRootDevice, which
// look up a specific resolved AMI id.
func TestDescribeImagesByID(t *testing.T) {
	f := New()
	f.Store.SeedImage(ec2types.Image{
		ImageId:      aws.String("ami-seeded-1"),
		CreationDate: aws.String("2025-01-01T00:00:00.000Z"),
		Name:         aws.String("custom/image"),
		Architecture: ec2types.ArchitectureValuesArm64,
	})
	out, err := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{ImageIds: []string{"ami-seeded-1"}})
	if err != nil {
		t.Fatalf("DescribeImages(byID): %v", err)
	}
	if len(out.Images) != 1 || aws.ToString(out.Images[0].ImageId) != "ami-seeded-1" {
		t.Fatalf("DescribeImages(byID) returned %+v", out.Images)
	}
	if out.Images[0].Architecture != ec2types.ArchitectureValuesArm64 {
		t.Fatalf("architecture = %q, want arm64", out.Images[0].Architecture)
	}
}

// TestSSMDefaultAndSeed protects the AMI resolver's SSM path: unseeded lookups
// return a permissive default, seeded values are honored.
func TestSSMDefaultAndSeed(t *testing.T) {
	f := New()
	def, err := f.SSM.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/aws/service/ami/x")})
	if err != nil {
		t.Fatalf("GetParameter(unseeded): %v", err)
	}
	if def.Parameter == nil || aws.ToString(def.Parameter.Value) != "ami-fake0000" {
		t.Fatalf("unseeded GetParameter value = %v, want ami-fake0000", def.Parameter)
	}

	f.Store.SeedParameter("/aws/service/ami/x", "ami-custom-9")
	got, _ := f.SSM.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/aws/service/ami/x")})
	if aws.ToString(got.Parameter.Value) != "ami-custom-9" {
		t.Fatalf("seeded GetParameter value = %q, want ami-custom-9", aws.ToString(got.Parameter.Value))
	}
}

// TestELBv2Lifecycle covers the NLB contract: ARN+DNS population (nlb.go:102),
// target-group ARN (nlb.go:159), registered-target health (nlb.go:369) and the
// not-found delete codes.
func TestELBv2Lifecycle(t *testing.T) {
	f := New()

	lb, err := f.ELBv2.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name: aws.String("env-nlb"),
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}
	if len(lb.LoadBalancers) != 1 {
		t.Fatalf("CreateLoadBalancer returned %d LBs, want 1", len(lb.LoadBalancers))
	}
	arn := aws.ToString(lb.LoadBalancers[0].LoadBalancerArn)
	if arn == "" || aws.ToString(lb.LoadBalancers[0].DNSName) == "" {
		t.Fatal("load balancer must populate LoadBalancerArn and DNSName")
	}
	if got := f.Store.ResourceCounts()["loadbalancers"]; got != 1 {
		t.Fatalf("loadbalancers count = %d, want 1", got)
	}

	tg, err := f.ELBv2.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name: aws.String("env-tg"), VpcId: aws.String("vpc-1"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}
	tgArn := aws.ToString(tg.TargetGroups[0].TargetGroupArn)
	if tgArn == "" {
		t.Fatal("target group must populate TargetGroupArn")
	}

	if _, err := f.ELBv2.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(tgArn),
		Targets:        []elbv2types.TargetDescription{{Id: aws.String("i-123"), Port: aws.Int32(6443)}},
	}); err != nil {
		t.Fatalf("RegisterTargets: %v", err)
	}
	health, err := f.ELBv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(tgArn),
	})
	if err != nil {
		t.Fatalf("DescribeTargetHealth: %v", err)
	}
	if len(health.TargetHealthDescriptions) != 1 || health.TargetHealthDescriptions[0].Target == nil ||
		aws.ToString(health.TargetHealthDescriptions[0].Target.Id) != "i-123" {
		t.Fatalf("DescribeTargetHealth must return registered target, got %+v", health.TargetHealthDescriptions)
	}

	lst, err := f.ELBv2.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(arn), Port: aws.Int32(6443),
	})
	if err != nil {
		t.Fatalf("CreateListener: %v", err)
	}
	if got := f.Store.ResourceCounts()["listeners"]; got != 1 {
		t.Fatalf("listeners count = %d, want 1", got)
	}

	// Delete everything and confirm the store empties.
	if _, err := f.ELBv2.DeleteListener(ctx, &elasticloadbalancingv2.DeleteListenerInput{
		ListenerArn: lst.Listeners[0].ListenerArn}); err != nil {
		t.Fatalf("DeleteListener: %v", err)
	}
	if _, err := f.ELBv2.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: aws.String(tgArn)}); err != nil {
		t.Fatalf("DeleteTargetGroup: %v", err)
	}
	if _, err := f.ELBv2.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(arn)}); err != nil {
		t.Fatalf("DeleteLoadBalancer: %v", err)
	}

	// Not-found delete codes the provider matches on.
	if _, err := f.ELBv2.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String("arn:gone")}); err == nil || !strings.Contains(err.Error(), "LoadBalancerNotFound") {
		t.Fatalf("DeleteLoadBalancer(absent) err = %v, want LoadBalancerNotFound", err)
	}
	if _, err := f.ELBv2.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: aws.String("arn:gone")}); err == nil || !strings.Contains(err.Error(), "TargetGroupNotFound") {
		t.Fatalf("DeleteTargetGroup(absent) err = %v, want TargetGroupNotFound", err)
	}
	if _, err := f.ELBv2.DeleteListener(ctx, &elasticloadbalancingv2.DeleteListenerInput{
		ListenerArn: aws.String("arn:gone")}); err == nil || !strings.Contains(err.Error(), "ListenerNotFound") {
		t.Fatalf("DeleteListener(absent) err = %v, want ListenerNotFound", err)
	}
}

// TestDescribeTargetGroupsFilterByLoadBalancer protects delete.go:86-91, where
// deleteNLBForCluster passes DescribeTargetGroupsInput.LoadBalancerArn and takes
// TargetGroups[0]. If the fake ignores the filter and a store holds >1 LB/TG,
// that [0] can be the wrong cluster's target group. The listener forwarding a LB
// to a TG is the association real AWS records in TargetGroup.LoadBalancerArns.
func TestDescribeTargetGroupsFilterByLoadBalancer(t *testing.T) {
	f := New()

	// Two independent clusters: each an LB + TG wired together by a listener,
	// exactly as createTargetGroup/createListener do in nlb.go.
	makeCluster := func(lbName, tgName string) (lbArn, tgArn string) {
		t.Helper()
		lb, err := f.ELBv2.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{Name: aws.String(lbName)})
		if err != nil {
			t.Fatalf("CreateLoadBalancer(%s): %v", lbName, err)
		}
		lbArn = aws.ToString(lb.LoadBalancers[0].LoadBalancerArn)
		tg, err := f.ELBv2.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{Name: aws.String(tgName), VpcId: aws.String("vpc-1")})
		if err != nil {
			t.Fatalf("CreateTargetGroup(%s): %v", tgName, err)
		}
		tgArn = aws.ToString(tg.TargetGroups[0].TargetGroupArn)
		if _, err := f.ELBv2.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
			LoadBalancerArn: aws.String(lbArn),
			DefaultActions: []elbv2types.Action{{
				Type: elbv2types.ActionTypeEnumForward,
				ForwardConfig: &elbv2types.ForwardActionConfig{
					TargetGroups: []elbv2types.TargetGroupTuple{{TargetGroupArn: aws.String(tgArn)}},
				},
			}},
		}); err != nil {
			t.Fatalf("CreateListener(%s): %v", lbName, err)
		}
		return lbArn, tgArn
	}

	lb1, tg1 := makeCluster("env-one-nlb", "env-one-tg")
	lb2, tg2 := makeCluster("env-two-nlb", "env-two-tg")

	// Filtering by a LB's ARN must return only that LB's target group.
	out, err := f.ELBv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		LoadBalancerArn: aws.String(lb1),
	})
	if err != nil {
		t.Fatalf("DescribeTargetGroups(lb1): %v", err)
	}
	if len(out.TargetGroups) != 1 || aws.ToString(out.TargetGroups[0].TargetGroupArn) != tg1 {
		t.Fatalf("DescribeTargetGroups(lb1) = %d groups %v, want exactly [tg1=%s]",
			len(out.TargetGroups), tgArns(out.TargetGroups), tg1)
	}

	out, err = f.ELBv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		LoadBalancerArn: aws.String(lb2),
	})
	if err != nil {
		t.Fatalf("DescribeTargetGroups(lb2): %v", err)
	}
	if len(out.TargetGroups) != 1 || aws.ToString(out.TargetGroups[0].TargetGroupArn) != tg2 {
		t.Fatalf("DescribeTargetGroups(lb2) = %d groups %v, want exactly [tg2=%s]",
			len(out.TargetGroups), tgArns(out.TargetGroups), tg2)
	}

	// No filter still returns every target group (behavior unchanged).
	out, err = f.ELBv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	if err != nil {
		t.Fatalf("DescribeTargetGroups(all): %v", err)
	}
	if len(out.TargetGroups) != 2 {
		t.Fatalf("DescribeTargetGroups(no filter) = %d groups, want 2", len(out.TargetGroups))
	}
}

func tgArns(tgs []elbv2types.TargetGroup) []string {
	out := make([]string, 0, len(tgs))
	for _, tg := range tgs {
		out = append(out, aws.ToString(tg.TargetGroupArn))
	}
	return out
}

// TestNatGatewayStates protects the create/delete poll loops (create.go /
// delete.go) that key on available/deleted NAT gateway states.
func TestNatGatewayStates(t *testing.T) {
	f := New()
	sub, _ := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{CidrBlock: aws.String("10.0.1.0/24")})
	nat, err := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: sub.Subnet.SubnetId})
	if err != nil {
		t.Fatalf("CreateNatGateway: %v", err)
	}
	natID := aws.ToString(nat.NatGateway.NatGatewayId)
	if nat.NatGateway.State != ec2types.NatGatewayStateAvailable {
		t.Fatalf("NAT gateway create state = %q, want available", nat.NatGateway.State)
	}
	desc, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{natID}})
	if len(desc.NatGateways) != 1 || desc.NatGateways[0].State != ec2types.NatGatewayStateAvailable {
		t.Fatalf("DescribeNatGateways must report available, got %+v", desc.NatGateways)
	}
	if _, err := f.EC2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(natID)}); err != nil {
		t.Fatalf("DeleteNatGateway: %v", err)
	}
	if got := f.Store.ResourceCounts()["natgateways"]; got != 0 {
		t.Fatalf("natgateways live count after delete = %d, want 0", got)
	}
}

// TestElasticIPRoundTrip covers AllocateAddress/ReleaseAddress and the
// idempotent release code.
func TestElasticIPRoundTrip(t *testing.T) {
	f := New()
	eip, err := f.EC2.AllocateAddress(ctx, &ec2.AllocateAddressInput{})
	if err != nil {
		t.Fatalf("AllocateAddress: %v", err)
	}
	allocID := aws.ToString(eip.AllocationId)
	if allocID == "" {
		t.Fatal("AllocateAddress must return an AllocationId")
	}
	if got := f.Store.ResourceCounts()["addresses"]; got != 1 {
		t.Fatalf("addresses count = %d, want 1", got)
	}
	if _, err := f.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(allocID)}); err != nil {
		t.Fatalf("ReleaseAddress: %v", err)
	}
	if _, err := f.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String("eipalloc-gone")}); err == nil ||
		!strings.Contains(err.Error(), "InvalidAllocationID.NotFound") {
		t.Fatalf("ReleaseAddress(absent) err = %v, want InvalidAllocationID.NotFound", err)
	}
}

// TestSecurityGroupRulesRoundTrip backs revokeSecurityGroupRules: authorized
// ingress must be readable via DescribeSecurityGroups, and DescribeSecurityGroups
// by absent id must error (so securityGroupExists returns false).
func TestSecurityGroupRulesRoundTrip(t *testing.T) {
	f := New()
	sg, _ := f.EC2.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName: aws.String("g"), VpcId: aws.String("vpc-1"),
	})
	sgID := aws.ToString(sg.GroupId)
	if _, err := f.EC2.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       aws.String(sgID),
		IpPermissions: []ec2types.IpPermission{{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22)}},
	}); err != nil {
		t.Fatalf("AuthorizeSecurityGroupIngress: %v", err)
	}
	desc, err := f.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{sgID}})
	if err != nil {
		t.Fatalf("DescribeSecurityGroups(existing): %v", err)
	}
	if len(desc.SecurityGroups) != 1 || len(desc.SecurityGroups[0].IpPermissions) != 1 {
		t.Fatalf("expected 1 SG with 1 ingress rule, got %+v", desc.SecurityGroups)
	}
	if _, err := f.EC2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String(sgID)}); err != nil {
		t.Fatalf("DeleteSecurityGroup: %v", err)
	}
	if _, err := f.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{sgID}}); err == nil {
		t.Fatal("DescribeSecurityGroups(deleted) must error so securityGroupExists()==false")
	}
}

func hasArch(it ec2types.InstanceTypeInfo, want ec2types.ArchitectureType) bool {
	if it.ProcessorInfo == nil {
		return false
	}
	return slices.Contains(it.ProcessorInfo.SupportedArchitectures, want)
}

// TestInputsRecordsCallArguments covers the input recorder the provider unit
// tests use to inspect the exact arguments passed to instrumented methods
// (e.g. the SG rules on AuthorizeSecurityGroupIngress, the route target on
// CreateRoute) — the capability the old per-method closures provided.
func TestInputsRecordsCallArguments(t *testing.T) {
	f := New()

	if got := f.Store.Inputs("CreateSubnet"); got != nil {
		t.Fatalf("Inputs before any call = %v, want nil", got)
	}

	if _, err := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId: aws.String("vpc-1"), CidrBlock: aws.String("10.0.9.0/24"),
	}); err != nil {
		t.Fatalf("CreateSubnet: %v", err)
	}
	if _, err := f.EC2.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId: aws.String("vpc-1"), CidrBlock: aws.String("10.0.8.0/24"),
	}); err != nil {
		t.Fatalf("CreateSubnet: %v", err)
	}

	in := f.Store.Inputs("CreateSubnet")
	if len(in) != 2 {
		t.Fatalf("Inputs(CreateSubnet) recorded %d calls, want 2", len(in))
	}
	// Recorded in call order, carrying the real input arguments.
	first, ok := in[0].(*ec2.CreateSubnetInput)
	if !ok {
		t.Fatalf("Inputs[0] type = %T, want *ec2.CreateSubnetInput", in[0])
	}
	if aws.ToString(first.CidrBlock) != "10.0.9.0/24" {
		t.Errorf("Inputs[0].CidrBlock = %q, want 10.0.9.0/24", aws.ToString(first.CidrBlock))
	}
	second := in[1].(*ec2.CreateSubnetInput)
	if aws.ToString(second.CidrBlock) != "10.0.8.0/24" {
		t.Errorf("Inputs[1].CidrBlock = %q, want 10.0.8.0/24", aws.ToString(second.CidrBlock))
	}
}

// TestSeedNextNatGatewayStatePendingThenAvailable covers the scripted NAT
// state used by createNATGateway's poll loop (create.go:662): the gateway
// reports pending for the seeded number of observations, then available.
func TestSeedNextNatGatewayStatePendingThenAvailable(t *testing.T) {
	f := New()
	f.Store.SeedNextNatGatewayState(1, ec2types.NatGatewayStateAvailable)

	nat, err := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-1")})
	if err != nil {
		t.Fatalf("CreateNatGateway: %v", err)
	}
	id := aws.ToString(nat.NatGateway.NatGatewayId)
	if nat.NatGateway.State != ec2types.NatGatewayStatePending {
		t.Fatalf("created NAT state = %q, want pending", nat.NatGateway.State)
	}

	d1, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id}})
	if len(d1.NatGateways) != 1 || d1.NatGateways[0].State != ec2types.NatGatewayStatePending {
		t.Fatalf("first describe state = %+v, want pending", d1.NatGateways)
	}
	d2, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id}})
	if len(d2.NatGateways) != 1 || d2.NatGateways[0].State != ec2types.NatGatewayStateAvailable {
		t.Fatalf("second describe state = %+v, want available", d2.NatGateways)
	}
}

// TestSeedNextNatGatewayStateFailed drives createNATGateway's failure branch
// (create.go:678); an unseeded NAT stays available by default.
func TestSeedNextNatGatewayStateFailed(t *testing.T) {
	f := New()
	f.Store.SeedNextNatGatewayState(0, ec2types.NatGatewayStateFailed)

	nat, err := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-1")})
	if err != nil {
		t.Fatalf("CreateNatGateway: %v", err)
	}
	id := aws.ToString(nat.NatGateway.NatGatewayId)
	d, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id}})
	if len(d.NatGateways) != 1 || d.NatGateways[0].State != ec2types.NatGatewayStateFailed {
		t.Fatalf("describe state = %+v, want failed", d.NatGateways)
	}

	// The scripting is one-shot: a second NAT is available by default.
	nat2, _ := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-2")})
	if nat2.NatGateway.State != ec2types.NatGatewayStateAvailable {
		t.Fatalf("second (unseeded) NAT state = %q, want available", nat2.NatGateway.State)
	}
}

// TestSeedInstanceTypeOverrides covers the filtered-DescribeInstanceTypes
// overrides the arch-inference provider tests rely on: explicit architectures
// (bypassing the prefix heuristic) and absent types (region-unavailable), while
// unseeded types keep the permissive-echo default.
func TestSeedInstanceTypeOverrides(t *testing.T) {
	f := New()
	f.Store.SeedInstanceTypeArchs("mac2-m2.metal", ec2types.ArchitectureTypeArm64Mac)
	f.Store.SeedInstanceTypeAbsent("t99.nonexistent")

	got, err := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{"mac2-m2.metal"},
	})
	if err != nil {
		t.Fatalf("DescribeInstanceTypes(mac2-m2.metal): %v", err)
	}
	if len(got.InstanceTypes) != 1 || !hasArch(got.InstanceTypes[0], ec2types.ArchitectureTypeArm64Mac) {
		t.Fatalf("mac2-m2.metal must report arm64_mac, got %+v", got.InstanceTypes)
	}

	absent, _ := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{"t99.nonexistent"},
	})
	if len(absent.InstanceTypes) != 0 {
		t.Fatalf("absent type must return no results, got %+v", absent.InstanceTypes)
	}

	// Unseeded types still echo via the prefix heuristic.
	echo, _ := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{"g5g.xlarge"},
	})
	if len(echo.InstanceTypes) != 1 || !hasArch(echo.InstanceTypes[0], ec2types.ArchitectureTypeArm64) {
		t.Fatalf("unseeded g5g.xlarge must still echo arm64, got %+v", echo.InstanceTypes)
	}
}

// TestSetImagesReplacesCatalog covers the catalog control the legacy-AMI
// provider tests rely on, including the empty "no AMIs found" case.
func TestSetImagesReplacesCatalog(t *testing.T) {
	f := New()
	def, _ := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{})
	if len(def.Images) == 0 {
		t.Fatal("default catalog should be seeded")
	}

	f.Store.SetImages(ec2types.Image{
		ImageId:      aws.String("ami-only"),
		Architecture: ec2types.ArchitectureValuesArm64,
	})
	out, _ := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{})
	if len(out.Images) != 1 || aws.ToString(out.Images[0].ImageId) != "ami-only" {
		t.Fatalf("SetImages must replace the catalog, got %+v", out.Images)
	}

	f.Store.SetImages()
	empty, _ := f.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{})
	if len(empty.Images) != 0 {
		t.Fatalf("SetImages() must clear the catalog, got %+v", empty.Images)
	}
}

// TestSeedNextNatGatewayDeleteState covers the teardown transition
// deleteNATGateway's wait-for-deleted poll loop relies on: a deleting gateway
// reports "deleting" for the seeded observations, then is removed.
func TestSeedNextNatGatewayDeleteState(t *testing.T) {
	f := New()
	nat, _ := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-1")})
	id := aws.ToString(nat.NatGateway.NatGatewayId)

	f.Store.SeedNextNatGatewayDeleteState(1)
	if _, err := f.EC2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(id)}); err != nil {
		t.Fatalf("DeleteNatGateway: %v", err)
	}

	d1, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id}})
	if len(d1.NatGateways) != 1 || d1.NatGateways[0].State != ec2types.NatGatewayStateDeleting {
		t.Fatalf("first describe = %+v, want deleting", d1.NatGateways)
	}
	d2, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id}})
	if len(d2.NatGateways) != 0 {
		t.Fatalf("second describe = %+v, want empty (removed)", d2.NatGateways)
	}

	// Default delete removes immediately.
	nat2, _ := f.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{SubnetId: aws.String("subnet-2")})
	id2 := aws.ToString(nat2.NatGateway.NatGatewayId)
	if _, err := f.EC2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(id2)}); err != nil {
		t.Fatalf("DeleteNatGateway(default): %v", err)
	}
	d3, _ := f.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{id2}})
	if len(d3.NatGateways) != 0 {
		t.Fatalf("default delete must remove immediately, got %+v", d3.NatGateways)
	}
}

// TestSeedDrainingENI covers the transition waitForENIsDrained re-polls on: a
// seeded interface reports in-use (blocking) for the seeded observations, then
// drains (removed).
func TestSeedDrainingENI(t *testing.T) {
	f := New()
	f.Store.SeedDrainingENI("vpc-1", 1)
	filt := &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{"vpc-1"}}},
	}

	d1, _ := f.EC2.DescribeNetworkInterfaces(ctx, filt)
	if len(d1.NetworkInterfaces) != 1 || d1.NetworkInterfaces[0].Status != ec2types.NetworkInterfaceStatusInUse {
		t.Fatalf("first poll = %+v, want one in-use ENI", d1.NetworkInterfaces)
	}
	d2, _ := f.EC2.DescribeNetworkInterfaces(ctx, filt)
	if len(d2.NetworkInterfaces) != 0 {
		t.Fatalf("second poll = %+v, want drained", d2.NetworkInterfaces)
	}
}

// TestSetInstanceTypeCatalog covers the no-filter catalog control that the
// checkInstanceTypes provider tests rely on, including the empty
// region-unavailable case.
func TestSetInstanceTypeCatalog(t *testing.T) {
	f := New()
	f.Store.SetInstanceTypeCatalog("t3.large", "t3.xlarge")

	out, err := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
	if err != nil {
		t.Fatalf("DescribeInstanceTypes(catalog): %v", err)
	}
	got := map[string]bool{}
	for _, it := range out.InstanceTypes {
		got[string(it.InstanceType)] = true
	}
	if !got["t3.large"] || !got["t3.xlarge"] {
		t.Fatalf("catalog must contain the set types, got %v", got)
	}
	if got["t3.medium"] {
		t.Fatalf("catalog must no longer contain the default t3.medium, got %v", got)
	}

	f.Store.SetInstanceTypeCatalog()
	empty, _ := f.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
	if len(empty.InstanceTypes) != 0 {
		t.Fatalf("empty catalog must return no types, got %+v", empty.InstanceTypes)
	}
}
