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
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCacheRoundTrip verifies that all AWS struct fields survive an
// updateStatus -> unmarsalCache round-trip. If a field is not persisted,
// delete will fail to clean up the corresponding resource.
func TestCacheRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-roundtrip-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cacheFile := filepath.Join(tmpDir, "cache.yaml")

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Region: "us-west-2",
			},
		},
	}

	log := logger.NewLogger()
	provider := &Provider{
		cacheFile:   cacheFile,
		Environment: &env,
		log:         log,
		sleep: noopSleep,
	}

	original := &AWS{
		Vpcid:                     "vpc-abc123",
		Subnetid:                  "subnet-def456",
		InternetGwid:              "igw-ghi789",
		InternetGatewayAttachment: "vpc-abc123",
		RouteTable:                "rtb-jkl012",
		SecurityGroupid:           "sg-mno345",
		Instanceid:                "i-pqr678",
		PublicDnsName:             "ec2-1-2-3-4.compute.amazonaws.com",
		// New cluster networking fields
		PublicSubnetid:        "subnet-pub789",
		NatGatewayid:          "nat-abc123",
		PublicRouteTable:      "rtb-pub456",
		CPSecurityGroupid:     "sg-cp789",
		WorkerSecurityGroupid: "sg-worker012",
		EIPAllocationid:       "eipalloc-345",
		IAMInstanceProfileArn: "arn:aws:iam::123456789012:instance-profile/holodeck",
	}

	// Write cache via updateStatus
	if err := provider.updateStatus(env, original, nil); err != nil {
		t.Fatalf("updateStatus failed: %v", err)
	}

	// Read it back
	restored, err := provider.unmarsalCache()
	if err != nil {
		t.Fatalf("unmarsalCache failed: %v", err)
	}

	// Verify every field survives the round-trip
	assertions := []struct {
		name     string
		got      string
		expected string
	}{
		{"Vpcid", restored.Vpcid, original.Vpcid},
		{"Subnetid", restored.Subnetid, original.Subnetid},
		{"InternetGwid", restored.InternetGwid, original.InternetGwid},
		{"InternetGatewayAttachment", restored.InternetGatewayAttachment, original.InternetGatewayAttachment},
		{"RouteTable", restored.RouteTable, original.RouteTable},
		{"SecurityGroupid", restored.SecurityGroupid, original.SecurityGroupid},
		{"Instanceid", restored.Instanceid, original.Instanceid},
		{"PublicDnsName", restored.PublicDnsName, original.PublicDnsName},
		{"PublicSubnetid", restored.PublicSubnetid, original.PublicSubnetid},
		{"NatGatewayid", restored.NatGatewayid, original.NatGatewayid},
		{"PublicRouteTable", restored.PublicRouteTable, original.PublicRouteTable},
		{"CPSecurityGroupid", restored.CPSecurityGroupid, original.CPSecurityGroupid},
		{"WorkerSecurityGroupid", restored.WorkerSecurityGroupid, original.WorkerSecurityGroupid},
		{"EIPAllocationid", restored.EIPAllocationid, original.EIPAllocationid},
		{"IAMInstanceProfileArn", restored.IAMInstanceProfileArn, original.IAMInstanceProfileArn},
	}

	for _, a := range assertions {
		if a.got != a.expected {
			t.Errorf("field %s: got %q, want %q", a.name, a.got, a.expected)
		}
	}
}

// TestCacheRoundTripSingleNode verifies backward compatibility: a cache
// with only the original single-node fields round-trips correctly.
func TestCacheRoundTripSingleNode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-single-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cacheFile := filepath.Join(tmpDir, "cache.yaml")

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-single"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Region: "us-east-1",
			},
		},
	}

	log := logger.NewLogger()
	provider := &Provider{
		cacheFile:   cacheFile,
		Environment: &env,
		log:         log,
		sleep: noopSleep,
	}

	// Only set the original single-node fields
	original := &AWS{
		Vpcid:                     "vpc-single",
		Subnetid:                  "subnet-single",
		InternetGwid:              "igw-single",
		InternetGatewayAttachment: "vpc-single",
		RouteTable:                "rtb-single",
		SecurityGroupid:           "sg-single",
		Instanceid:                "i-single",
		PublicDnsName:             "ec2-single.compute.amazonaws.com",
	}

	if err := provider.updateStatus(env, original, nil); err != nil {
		t.Fatalf("updateStatus failed: %v", err)
	}

	restored, err := provider.unmarsalCache()
	if err != nil {
		t.Fatalf("unmarsalCache failed: %v", err)
	}

	// New fields should be empty strings
	if restored.PublicSubnetid != "" {
		t.Errorf("PublicSubnetid should be empty, got %q", restored.PublicSubnetid)
	}
	if restored.NatGatewayid != "" {
		t.Errorf("NatGatewayid should be empty, got %q", restored.NatGatewayid)
	}

	// Original fields must survive
	if restored.Vpcid != original.Vpcid {
		t.Errorf("Vpcid: got %q, want %q", restored.Vpcid, original.Vpcid)
	}
	if restored.SecurityGroupid != original.SecurityGroupid {
		t.Errorf("SecurityGroupid: got %q, want %q", restored.SecurityGroupid, original.SecurityGroupid)
	}
}
