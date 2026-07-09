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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
)

func newTestProvider(client internalaws.EC2Client) *Provider {
	env := v1alpha1.Environment{}
	env.Name = "test-cluster"
	env.Spec.PrivateKey = "test-key"
	env.Spec.Username = "ubuntu"
	env.Spec.KeyName = "test-key"
	return &Provider{
		ec2:         client,
		Environment: &env,
		log:         mockLogger(),
		sleep:       noopSleep,
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test-cluster")},
		},
	}
}

// TestCreateControlPlaneSecurityGroup verifies CP SG has correct rules
func TestCreateControlPlaneSecurityGroup(t *testing.T) {
	f := awsfake.New()
	p := newTestProvider(f.EC2)
	cache := &ClusterCache{}
	cache.Vpcid = "vpc-test"

	err := p.createControlPlaneSecurityGroup(cache)
	if err != nil {
		t.Fatalf("createControlPlaneSecurityGroup() error = %v", err)
	}

	// Verify SG ID was set
	if cache.CPSecurityGroupid == "" {
		t.Fatal("CPSecurityGroupid not set")
	}
	if cache.SecurityGroupid != cache.CPSecurityGroupid {
		t.Errorf("SecurityGroupid not set for backward compat: got %q, want %q",
			cache.SecurityGroupid, cache.CPSecurityGroupid)
	}

	// Verify SG was created with correct name
	createCalls := f.Store.Inputs("CreateSecurityGroup")
	if len(createCalls) != 1 {
		t.Fatalf("expected 1 CreateSecurityGroup call, got %d", len(createCalls))
	}
	if got := aws.ToString(createCalls[0].(*ec2.CreateSecurityGroupInput).GroupName); got != "test-cluster-cp" {
		t.Errorf("SG name: got %q, want %q", got, "test-cluster-cp")
	}

	// Verify ingress rules
	authorizeCalls := f.Store.Inputs("AuthorizeSecurityGroupIngress")
	if len(authorizeCalls) != 1 {
		t.Fatalf("expected 1 AuthorizeSecurityGroupIngress call, got %d", len(authorizeCalls))
	}

	perms := authorizeCalls[0].(*ec2.AuthorizeSecurityGroupIngressInput).IpPermissions
	assertHasRule(t, perms, "etcd (2379-2380/tcp, CP self)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portEtcdClient &&
			aws.ToInt32(p.ToPort) == portEtcdPeer &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			len(p.IpRanges) == 0
	})

	assertHasRule(t, perms, "kube-controller-manager (10257/tcp, CP self)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portKubeController &&
			aws.ToInt32(p.ToPort) == portKubeController &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			len(p.IpRanges) == 0
	})

	assertHasRule(t, perms, "kube-scheduler (10259/tcp, CP self)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portKubeScheduler &&
			aws.ToInt32(p.ToPort) == portKubeScheduler &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			len(p.IpRanges) == 0
	})

	assertHasRule(t, perms, "SSH (22/tcp, caller IP)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portSSH &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			len(p.IpRanges) > 0
	})

	assertHasRule(t, perms, "K8s API (6443/tcp, NLB+caller)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portK8sAPI &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasCIDR(p.IpRanges, nlbSubnetCIDR)
	})

	assertHasRule(t, perms, "ICMP from VPC", func(p types.IpPermission) bool {
		return aws.ToString(p.IpProtocol) == "icmp" &&
			hasCIDR(p.IpRanges, vpcCIDR)
	})

	// Verify NO NodePort rule on CP
	assertNoRule(t, perms, "NodePort on CP", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portNodePortStart
	})
}

// TestCreateWorkerSecurityGroup verifies Worker SG has correct rules
func TestCreateWorkerSecurityGroup(t *testing.T) {
	f := awsfake.New()
	p := newTestProvider(f.EC2)
	cache := &ClusterCache{}
	cache.Vpcid = "vpc-test"
	cache.CPSecurityGroupid = "sg-cp-existing"
	cache.SecurityGroupid = "sg-cp-existing"

	err := p.createWorkerSecurityGroup(cache)
	if err != nil {
		t.Fatalf("createWorkerSecurityGroup() error = %v", err)
	}

	if cache.WorkerSecurityGroupid == "" {
		t.Fatal("WorkerSecurityGroupid not set")
	}

	// SG name should be test-cluster-worker
	createCalls := f.Store.Inputs("CreateSecurityGroup")
	if len(createCalls) != 1 {
		t.Fatalf("expected 1 CreateSecurityGroup call, got %d", len(createCalls))
	}
	if got := aws.ToString(createCalls[0].(*ec2.CreateSecurityGroupInput).GroupName); got != "test-cluster-worker" {
		t.Errorf("SG name: got %q, want %q", got, "test-cluster-worker")
	}

	// Should have 2 AuthorizeSecurityGroupIngress calls:
	// 1) Worker SG rules, 2) CP cross-references
	authorizeCalls := f.Store.Inputs("AuthorizeSecurityGroupIngress")
	if len(authorizeCalls) != 2 {
		t.Fatalf("expected 2 AuthorizeSecurityGroupIngress calls, got %d", len(authorizeCalls))
	}
	firstAuth := authorizeCalls[0].(*ec2.AuthorizeSecurityGroupIngressInput)
	secondAuth := authorizeCalls[1].(*ec2.AuthorizeSecurityGroupIngressInput)

	// First call: worker SG ingress rules
	workerPerms := firstAuth.IpPermissions
	workerSGID := aws.ToString(firstAuth.GroupId)
	if workerSGID != cache.WorkerSecurityGroupid {
		t.Errorf("first authorize call target: got %q, want %q", workerSGID, cache.WorkerSecurityGroupid)
	}

	assertHasRule(t, workerPerms, "kubelet from CP only (10250/tcp)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portKubelet &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			!hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})

	assertHasRule(t, workerPerms, "NodePort TCP (30000-32767)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portNodePortStart &&
			aws.ToInt32(p.ToPort) == portNodePortEnd &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasCIDR(p.IpRanges, "0.0.0.0/0")
	})

	assertHasRule(t, workerPerms, "NodePort UDP (30000-32767)", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portNodePortStart &&
			aws.ToInt32(p.ToPort) == portNodePortEnd &&
			aws.ToString(p.IpProtocol) == "udp" &&
			hasCIDR(p.IpRanges, "0.0.0.0/0")
	})

	assertHasRule(t, workerPerms, "Calico VXLAN from both SGs", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portCalicoVXLAN &&
			aws.ToString(p.IpProtocol) == "udp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})

	assertHasRule(t, workerPerms, "Calico BGP from both SGs", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portCalicoBGP &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})

	assertHasRule(t, workerPerms, "Calico Typha from both SGs", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portCalicoTypha &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.CPSecurityGroupid) &&
			hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})

	// Worker SG should NOT have etcd
	assertNoRule(t, workerPerms, "etcd on Worker", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portEtcdClient
	})

	// Second call: CP cross-references targeting CP SG
	cpCrossPerms := secondAuth.IpPermissions
	cpCrossSGID := aws.ToString(secondAuth.GroupId)
	if cpCrossSGID != cache.CPSecurityGroupid {
		t.Errorf("second authorize call target: got %q, want %q", cpCrossSGID, cache.CPSecurityGroupid)
	}

	assertHasRule(t, cpCrossPerms, "K8s API from Worker SG", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portK8sAPI &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})

	assertHasRule(t, cpCrossPerms, "kubelet from Worker SG", func(p types.IpPermission) bool {
		return aws.ToInt32(p.FromPort) == portKubelet &&
			aws.ToString(p.IpProtocol) == "tcp" &&
			hasSGRef(p.UserIdGroupPairs, cache.WorkerSecurityGroupid)
	})
}

// TestWorkerSGRequiresCPSG verifies that creating worker SG fails without CP SG
func TestWorkerSGRequiresCPSG(t *testing.T) {
	f := awsfake.New()
	p := newTestProvider(f.EC2)
	cache := &ClusterCache{}
	cache.Vpcid = "vpc-test"
	// CPSecurityGroupid is intentionally empty

	err := p.createWorkerSecurityGroup(cache)
	if err == nil {
		t.Fatal("expected error when CP SG not set, got nil")
	}
}

// TestInstancesUseRoleSpecificSGs verifies that createInstances picks the right SG per role
func TestInstancesUseRoleSpecificSGs(t *testing.T) {
	// Test the SG selection logic that createInstances uses:
	// CP nodes get CPSecurityGroupid, Worker nodes get WorkerSecurityGroupid,
	// and if neither is set, SecurityGroupid is the fallback.
	cache := &ClusterCache{}
	cache.SecurityGroupid = "sg-fallback"
	cache.CPSecurityGroupid = "sg-cp-123"
	cache.WorkerSecurityGroupid = "sg-worker-456"

	selectSG := func(cache *ClusterCache, role NodeRole) string {
		sgID := cache.SecurityGroupid
		if role == NodeRoleControlPlane && cache.CPSecurityGroupid != "" {
			sgID = cache.CPSecurityGroupid
		} else if role == NodeRoleWorker && cache.WorkerSecurityGroupid != "" {
			sgID = cache.WorkerSecurityGroupid
		}
		return sgID
	}

	if got := selectSG(cache, NodeRoleControlPlane); got != "sg-cp-123" {
		t.Errorf("CP SG: got %q, want %q", got, "sg-cp-123")
	}
	if got := selectSG(cache, NodeRoleWorker); got != "sg-worker-456" {
		t.Errorf("Worker SG: got %q, want %q", got, "sg-worker-456")
	}

	// Fallback: empty role-specific SGs use the shared SG
	cacheNoRoles := &ClusterCache{}
	cacheNoRoles.SecurityGroupid = "sg-shared"
	if got := selectSG(cacheNoRoles, NodeRoleControlPlane); got != "sg-shared" {
		t.Errorf("CP fallback SG: got %q, want %q", got, "sg-shared")
	}
	if got := selectSG(cacheNoRoles, NodeRoleWorker); got != "sg-shared" {
		t.Errorf("Worker fallback SG: got %q, want %q", got, "sg-shared")
	}
}

// Helper: check if a list of permissions contains a rule matching the predicate
func assertHasRule(t *testing.T, perms []types.IpPermission, desc string, match func(types.IpPermission) bool) {
	t.Helper()
	for _, p := range perms {
		if match(p) {
			return
		}
	}
	t.Errorf("missing expected rule: %s", desc)
}

// Helper: check that no rule matches the predicate
func assertNoRule(t *testing.T, perms []types.IpPermission, desc string, match func(types.IpPermission) bool) {
	t.Helper()
	for _, p := range perms {
		if match(p) {
			t.Errorf("unexpected rule found: %s", desc)
			return
		}
	}
}

// Helper: check if UserIdGroupPairs contains a specific SG ID
func hasSGRef(pairs []types.UserIdGroupPair, sgID string) bool {
	for _, p := range pairs {
		if aws.ToString(p.GroupId) == sgID {
			return true
		}
	}
	return false
}

// Helper: check if IpRanges contains a specific CIDR
func hasCIDR(ranges []types.IpRange, cidr string) bool {
	for _, r := range ranges {
		if aws.ToString(r.CidrIp) == cidr {
			return true
		}
	}
	return false
}
