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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	internalaws "github.com/NVIDIA/holodeck/internal/aws"
)

// FakeELBv2 is a stateful in-memory implementation of internalaws.ELBv2Client.
type FakeELBv2 struct {
	store *Store
}

var _ internalaws.ELBv2Client = (*FakeELBv2)(nil)

const elbv2ARNPrefix = "arn:aws:elasticloadbalancing:us-east-1:123456789012"

// ---- Load Balancer ----

func (f *FakeELBv2) CreateLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.CreateLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateLoadBalancerOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateLoadBalancer", params)
	if err := f.store.failure("CreateLoadBalancer"); err != nil {
		return nil, err
	}
	name := aws.ToString(params.Name)
	id := f.store.nextID("lb")
	arn := fmt.Sprintf("%s:loadbalancer/net/%s/%s", elbv2ARNPrefix, name, id)
	lb := elbv2types.LoadBalancer{
		LoadBalancerArn:  aws.String(arn),
		LoadBalancerName: params.Name,
		DNSName:          aws.String(fmt.Sprintf("%s-%s.elb.amazonaws.com", name, id)),
		Type:             params.Type,
		Scheme:           params.Scheme,
		State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
	}
	f.store.LoadBalancers[arn] = &lb
	return &elasticloadbalancingv2.CreateLoadBalancerOutput{LoadBalancers: []elbv2types.LoadBalancer{lb}}, nil
}

func (f *FakeELBv2) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeLoadBalancers", params)
	if err := f.store.failure("DescribeLoadBalancers"); err != nil {
		return nil, err
	}
	var out []elbv2types.LoadBalancer
	switch {
	case len(params.LoadBalancerArns) > 0:
		for _, arn := range params.LoadBalancerArns {
			lb, ok := f.store.LoadBalancers[arn]
			if !ok {
				return nil, fmt.Errorf("LoadBalancerNotFound: %s", arn)
			}
			out = append(out, *lb)
		}
	case len(params.Names) > 0:
		for _, name := range params.Names {
			found := false
			for _, lb := range f.store.LoadBalancers {
				if aws.ToString(lb.LoadBalancerName) == name {
					out = append(out, *lb)
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("LoadBalancerNotFound: %s", name)
			}
		}
	default:
		for _, lb := range f.store.LoadBalancers {
			out = append(out, *lb)
		}
	}
	return &elasticloadbalancingv2.DescribeLoadBalancersOutput{LoadBalancers: out}, nil
}

func (f *FakeELBv2) DeleteLoadBalancer(ctx context.Context, params *elasticloadbalancingv2.DeleteLoadBalancerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteLoadBalancerOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteLoadBalancer", params)
	if err := f.store.failure("DeleteLoadBalancer"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.LoadBalancerArn)
	if _, ok := f.store.LoadBalancers[arn]; !ok {
		return nil, fmt.Errorf("LoadBalancerNotFound: %s", arn)
	}
	delete(f.store.LoadBalancers, arn)
	return &elasticloadbalancingv2.DeleteLoadBalancerOutput{}, nil
}

// ---- Target Group ----

func (f *FakeELBv2) CreateTargetGroup(ctx context.Context, params *elasticloadbalancingv2.CreateTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateTargetGroupOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateTargetGroup", params)
	if err := f.store.failure("CreateTargetGroup"); err != nil {
		return nil, err
	}
	name := aws.ToString(params.Name)
	id := f.store.nextID("tg")
	arn := fmt.Sprintf("%s:targetgroup/%s/%s", elbv2ARNPrefix, name, id)
	tg := elbv2types.TargetGroup{
		TargetGroupArn:  aws.String(arn),
		TargetGroupName: params.Name,
		VpcId:           params.VpcId,
		Port:            params.Port,
		Protocol:        params.Protocol,
	}
	f.store.TargetGroups[arn] = &tg
	return &elasticloadbalancingv2.CreateTargetGroupOutput{TargetGroups: []elbv2types.TargetGroup{tg}}, nil
}

func (f *FakeELBv2) DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeTargetGroups", params)
	if err := f.store.failure("DescribeTargetGroups"); err != nil {
		return nil, err
	}
	var out []elbv2types.TargetGroup
	switch {
	case len(params.TargetGroupArns) > 0:
		for _, arn := range params.TargetGroupArns {
			tg, ok := f.store.TargetGroups[arn]
			if !ok {
				return nil, fmt.Errorf("TargetGroupNotFound: %s", arn)
			}
			out = append(out, *tg)
		}
	case params.LoadBalancerArn != nil:
		// The lookup used by deleteNLBForCluster (delete.go:86): only target
		// groups attached to this load balancer, matching real AWS which
		// filters on TargetGroup.LoadBalancerArns (populated at CreateListener).
		lbArn := aws.ToString(params.LoadBalancerArn)
		for _, tg := range f.store.TargetGroups {
			if slices.Contains(tg.LoadBalancerArns, lbArn) {
				out = append(out, *tg)
			}
		}
	default:
		for _, tg := range f.store.TargetGroups {
			out = append(out, *tg)
		}
	}
	return &elasticloadbalancingv2.DescribeTargetGroupsOutput{TargetGroups: out}, nil
}

func (f *FakeELBv2) DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeTargetHealth", params)
	if err := f.store.failure("DescribeTargetHealth"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.TargetGroupArn)
	if _, ok := f.store.TargetGroups[arn]; !ok {
		return nil, fmt.Errorf("TargetGroupNotFound: %s", arn)
	}
	var descs []elbv2types.TargetHealthDescription
	for _, target := range f.store.RegisteredTargets[arn] {
		descs = append(descs, elbv2types.TargetHealthDescription{
			Target:       &target,
			TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
		})
	}
	return &elasticloadbalancingv2.DescribeTargetHealthOutput{TargetHealthDescriptions: descs}, nil
}

func (f *FakeELBv2) DeleteTargetGroup(ctx context.Context, params *elasticloadbalancingv2.DeleteTargetGroupInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteTargetGroupOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteTargetGroup", params)
	if err := f.store.failure("DeleteTargetGroup"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.TargetGroupArn)
	if _, ok := f.store.TargetGroups[arn]; !ok {
		return nil, fmt.Errorf("TargetGroupNotFound: %s", arn)
	}
	delete(f.store.TargetGroups, arn)
	delete(f.store.RegisteredTargets, arn)
	return &elasticloadbalancingv2.DeleteTargetGroupOutput{}, nil
}

// ---- Target registration ----

func (f *FakeELBv2) RegisterTargets(ctx context.Context, params *elasticloadbalancingv2.RegisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RegisterTargetsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("RegisterTargets", params)
	if err := f.store.failure("RegisterTargets"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.TargetGroupArn)
	f.store.RegisteredTargets[arn] = append(f.store.RegisteredTargets[arn], params.Targets...)
	return &elasticloadbalancingv2.RegisterTargetsOutput{}, nil
}

func (f *FakeELBv2) DeregisterTargets(ctx context.Context, params *elasticloadbalancingv2.DeregisterTargetsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeregisterTargetsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeregisterTargets", params)
	if err := f.store.failure("DeregisterTargets"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.TargetGroupArn)
	remove := map[string]bool{}
	for _, target := range params.Targets {
		remove[aws.ToString(target.Id)] = true
	}
	var kept []elbv2types.TargetDescription
	for _, target := range f.store.RegisteredTargets[arn] {
		if !remove[aws.ToString(target.Id)] {
			kept = append(kept, target)
		}
	}
	f.store.RegisteredTargets[arn] = kept
	return &elasticloadbalancingv2.DeregisterTargetsOutput{}, nil
}

// ---- Listener ----

func (f *FakeELBv2) CreateListener(ctx context.Context, params *elasticloadbalancingv2.CreateListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateListenerOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("CreateListener", params)
	if err := f.store.failure("CreateListener"); err != nil {
		return nil, err
	}
	id := f.store.nextID("listener")
	arn := fmt.Sprintf("%s:listener/net/%s", elbv2ARNPrefix, id)
	l := elbv2types.Listener{
		ListenerArn:     aws.String(arn),
		LoadBalancerArn: params.LoadBalancerArn,
		Port:            params.Port,
		Protocol:        params.Protocol,
	}
	f.store.Listeners[arn] = &l

	// Forwarding a load balancer to a target group is the association real AWS
	// records in TargetGroup.LoadBalancerArns; DescribeTargetGroups(LoadBalancerArn)
	// filters on it. Record it here so that filter works (delete.go:86).
	lbArn := aws.ToString(params.LoadBalancerArn)
	for _, action := range params.DefaultActions {
		var tgArns []string
		if action.TargetGroupArn != nil {
			tgArns = append(tgArns, aws.ToString(action.TargetGroupArn))
		}
		if action.ForwardConfig != nil {
			for _, tgt := range action.ForwardConfig.TargetGroups {
				tgArns = append(tgArns, aws.ToString(tgt.TargetGroupArn))
			}
		}
		for _, tgArn := range tgArns {
			if tg, ok := f.store.TargetGroups[tgArn]; ok && !slices.Contains(tg.LoadBalancerArns, lbArn) {
				tg.LoadBalancerArns = append(tg.LoadBalancerArns, lbArn)
			}
		}
	}

	return &elasticloadbalancingv2.CreateListenerOutput{Listeners: []elbv2types.Listener{l}}, nil
}

func (f *FakeELBv2) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DescribeListeners", params)
	if err := f.store.failure("DescribeListeners"); err != nil {
		return nil, err
	}
	var out []elbv2types.Listener
	lbArn := aws.ToString(params.LoadBalancerArn)
	for _, l := range f.store.Listeners {
		if lbArn == "" || aws.ToString(l.LoadBalancerArn) == lbArn {
			out = append(out, *l)
		}
	}
	return &elasticloadbalancingv2.DescribeListenersOutput{Listeners: out}, nil
}

func (f *FakeELBv2) DeleteListener(ctx context.Context, params *elasticloadbalancingv2.DeleteListenerInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteListenerOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("DeleteListener", params)
	if err := f.store.failure("DeleteListener"); err != nil {
		return nil, err
	}
	arn := aws.ToString(params.ListenerArn)
	if _, ok := f.store.Listeners[arn]; !ok {
		return nil, fmt.Errorf("ListenerNotFound: %s", arn)
	}
	delete(f.store.Listeners, arn)
	return &elasticloadbalancingv2.DeleteListenerOutput{}, nil
}

// ---- Tags ----

func (f *FakeELBv2) AddTags(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.record("AddTags", params)
	if err := f.store.failure("AddTags"); err != nil {
		return nil, err
	}
	return &elasticloadbalancingv2.AddTagsOutput{}, nil
}
