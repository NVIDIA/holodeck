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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
)

// TestDeleteNLB_LoadBalancerNotFound: deleteNLB must treat a
// LoadBalancerNotFound from DeleteLoadBalancer as success (the LB is already
// gone). The fake returns that code for an absent ARN, so no injection needed.
func TestDeleteNLB_LoadBalancerNotFound(t *testing.T) {
	f := awsfake.New()
	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/net/gone/abc"}

	if err := provider.deleteNLB(cache); err != nil {
		t.Fatalf("expected no error when NLB is already deleted, got: %v", err)
	}
}

// TestDeleteNLB_RealError: a non-NotFound DeleteLoadBalancer error must
// propagate.
func TestDeleteNLB_RealError(t *testing.T) {
	f := awsfake.New()
	f.Store.FailNext("DeleteLoadBalancer", fmt.Errorf("InternalError: something went wrong"))
	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/net/test/abc"}

	if err := provider.deleteNLB(cache); err == nil {
		t.Fatal("expected error for InternalError, got nil")
	}
}

// TestDeleteListener_ListenerNotFound: deleteListener describes the LB's
// listeners then deletes each; a ListenerNotFound during delete is tolerated.
func TestDeleteListener_ListenerNotFound(t *testing.T) {
	f := awsfake.New()
	lb, err := f.ELBv2.CreateLoadBalancer(context.Background(), &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name: aws.String("test-nlb"),
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}
	lbArn := aws.ToString(lb.LoadBalancers[0].LoadBalancerArn)
	if _, err := f.ELBv2.CreateListener(context.Background(), &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(lbArn),
	}); err != nil {
		t.Fatalf("CreateListener: %v", err)
	}
	f.Store.FailNext("DeleteListener", fmt.Errorf("ListenerNotFound: One or more listeners not found"))

	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: lbArn}

	if err := provider.deleteListener(cache); err != nil {
		t.Fatalf("expected no error when listener is already deleted, got: %v", err)
	}
}

// TestDeleteListener_DescribeNotFound: a LoadBalancerNotFound while describing
// listeners means the LB is already gone; deleteListener tolerates it.
func TestDeleteListener_DescribeNotFound(t *testing.T) {
	f := awsfake.New()
	f.Store.FailNext("DescribeListeners", fmt.Errorf("LoadBalancerNotFound: LB already gone"))
	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{LoadBalancerArn: "arn:lb/gone"}

	if err := provider.deleteListener(cache); err != nil {
		t.Fatalf("expected no error when LB is already deleted during describe, got: %v", err)
	}
}

// TestDeleteTargetGroup_NotFound: an absent target group makes
// DescribeTargetHealth return TargetGroupNotFound, which deleteTargetGroup
// treats as success (already deleted).
func TestDeleteTargetGroup_NotFound(t *testing.T) {
	f := awsfake.New()
	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{TargetGroupArn: "arn:tg/gone"}

	if err := provider.deleteTargetGroup(cache); err != nil {
		t.Fatalf("expected no error when target group is already deleted, got: %v", err)
	}
}

// TestDeleteTargetGroup_RealError: with the target group present (health
// describe succeeds), a non-NotFound DeleteTargetGroup error must propagate.
func TestDeleteTargetGroup_RealError(t *testing.T) {
	f := awsfake.New()
	tg, err := f.ELBv2.CreateTargetGroup(context.Background(), &elasticloadbalancingv2.CreateTargetGroupInput{
		Name: aws.String("test-tg"), VpcId: aws.String("vpc-1"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}
	tgArn := aws.ToString(tg.TargetGroups[0].TargetGroupArn)
	f.Store.FailNext("DeleteTargetGroup", fmt.Errorf("InternalError: something went wrong"))

	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep}
	cache := &ClusterCache{TargetGroupArn: tgArn}

	if err := provider.deleteTargetGroup(cache); err == nil {
		t.Fatal("expected error for InternalError, got nil")
	}
}

// TestDeleteNLBForCluster_DescribeNotFound: the env-derived LB name is absent,
// so DescribeLoadBalancers(Names) returns LoadBalancerNotFound, which
// deleteNLBForCluster tolerates.
func TestDeleteNLBForCluster_DescribeNotFound(t *testing.T) {
	f := awsfake.New()
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
	}
	provider := &Provider{elbv2: f.ELBv2, log: mockLogger(), sleep: noopSleep, Environment: env}
	cache := &ClusterCache{LoadBalancerDNS: "gone-nlb.elb.amazonaws.com"}

	if err := provider.deleteNLBForCluster(cache); err != nil {
		t.Fatalf("expected no error when NLB is already deleted, got: %v", err)
	}
}

// orderRecordingELBv2 wraps a fake ELBv2 client to capture the cross-method
// call order of deleteNLB's three teardown calls. awsfake.Store's recorder
// (Inputs/CallsTo) only tracks per-method call order/counts, not a
// cross-method timeline, so this thin test-local decorator (no change to
// awsfake) is what makes the actual sequence observable.
type orderRecordingELBv2 struct {
	*awsfake.FakeELBv2
	order *[]string
}

func (o *orderRecordingELBv2) DeleteListener(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error) {
	*o.order = append(*o.order, "DeleteListener")
	return o.FakeELBv2.DeleteListener(ctx, params, optFns...)
}

func (o *orderRecordingELBv2) DeleteTargetGroup(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
	*o.order = append(*o.order, "DeleteTargetGroup")
	return o.FakeELBv2.DeleteTargetGroup(ctx, params, optFns...)
}

func (o *orderRecordingELBv2) DeleteLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
	*o.order = append(*o.order, "DeleteLoadBalancer")
	return o.FakeELBv2.DeleteLoadBalancer(ctx, params, optFns...)
}

// TestDeleteNLB_TeardownOrder pins the bug caught by nlb.go:266-297: deleteNLB
// must tear down listener -> target group -> load balancer in that order.
// Reversing the order (or skipping the listener stage) leaves a dangling
// listener/target-group reference that real AWS rejects with
// ResourceInUse — a defect this fake's unconditional deletes cannot surface
// on their own, since it never checks for lingering references.
func TestDeleteNLB_TeardownOrder(t *testing.T) {
	f := awsfake.New()

	lbOut, err := f.ELBv2.CreateLoadBalancer(context.Background(), &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name: aws.String("test-nlb"),
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}
	lbArn := aws.ToString(lbOut.LoadBalancers[0].LoadBalancerArn)

	tgOut, err := f.ELBv2.CreateTargetGroup(context.Background(), &elasticloadbalancingv2.CreateTargetGroupInput{
		Name: aws.String("test-tg"), VpcId: aws.String("vpc-1"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}
	tgArn := aws.ToString(tgOut.TargetGroups[0].TargetGroupArn)

	if _, err := f.ELBv2.CreateListener(context.Background(), &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(lbArn),
	}); err != nil {
		t.Fatalf("CreateListener: %v", err)
	}

	if _, err := f.ELBv2.RegisterTargets(context.Background(), &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(tgArn),
		Targets:        []elbv2types.TargetDescription{{Id: aws.String("i-fake0001")}},
	}); err != nil {
		t.Fatalf("RegisterTargets: %v", err)
	}

	var order []string
	provider := &Provider{
		elbv2: &orderRecordingELBv2{FakeELBv2: f.ELBv2, order: &order},
		log:   mockLogger(),
		sleep: noopSleep,
	}
	cache := &ClusterCache{LoadBalancerArn: lbArn, TargetGroupArn: tgArn}

	if err := provider.deleteNLB(cache); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"DeleteListener", "DeleteTargetGroup", "DeleteLoadBalancer"}
	if len(order) != len(want) {
		t.Fatalf("expected teardown order %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("expected teardown order %v, got %v", want, order)
		}
	}
}
