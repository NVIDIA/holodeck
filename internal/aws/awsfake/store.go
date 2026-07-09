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

// Package awsfake provides a stateful, in-memory fake of the AWS EC2, ELBv2 and
// SSM clients used by the holodeck AWS provider. It implements the
// internal/aws.{EC2Client,ELBv2Client,SSMClient} interfaces so it can be
// injected via the provider's WithEC2Client/WithSSMClient/WithELBv2Client
// options, letting the real provisioning/teardown code run credential-free
// against an observable resource store.
//
// The fake is AWS's documented interface-mock pattern (AWS ships no official Go
// SDK v2 mocks): https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html
package awsfake

import (
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// Fake bundles the three fake clients over one shared Store so a single test
// can drive create/describe/delete across EC2, ELBv2 and SSM and observe the
// resulting resource state.
type Fake struct {
	Store *Store
	EC2   *FakeEC2
	ELBv2 *FakeELBv2
	SSM   *FakeSSM
}

// New returns a Fake with permissive, seeded defaults: a small image catalog
// (including two Ubuntu images with distinct creation dates), an instance-type
// catalog covering the repo's test configs, and empty resource maps.
func New() *Fake {
	s := newStore()
	return &Fake{
		Store: s,
		EC2:   &FakeEC2{store: s},
		ELBv2: &FakeELBv2{store: s},
		SSM:   &FakeSSM{store: s},
	}
}

// Store is the in-memory resource database shared by the fake clients. All
// access is guarded by mu; the fake client methods lock it at entry, so the
// unexported record/failure/nextID helpers assume the lock is already held.
type Store struct {
	mu sync.Mutex

	// EC2 resources.
	Vpcs              map[string]*ec2types.Vpc
	Subnets           map[string]*ec2types.Subnet
	InternetGateways  map[string]*ec2types.InternetGateway
	RouteTables       map[string]*ec2types.RouteTable
	SecurityGroups    map[string]*ec2types.SecurityGroup
	Instances         map[string]*ec2types.Instance
	NatGateways       map[string]*ec2types.NatGateway
	Addresses         map[string]*ec2types.Address
	NetworkInterfaces map[string]*ec2types.NetworkInterface

	// ELBv2 resources (load balancers/target groups/listeners keyed by ARN).
	LoadBalancers     map[string]*elbv2types.LoadBalancer
	TargetGroups      map[string]*elbv2types.TargetGroup
	Listeners         map[string]*elbv2types.Listener
	RegisteredTargets map[string][]elbv2types.TargetDescription

	// SSM parameters.
	Parameters map[string]string

	// Seed data (excluded from ResourceCounts/Empty).
	Images        []ec2types.Image
	InstanceTypes map[string][]ec2types.ArchitectureType

	// Per-instance-type overrides for filtered DescribeInstanceTypes queries:
	// explicit architectures (bypassing the prefix heuristic) and types marked
	// as not offered (so a filtered query returns no results).
	instanceTypeArchs   map[string][]ec2types.ArchitectureType
	absentInstanceTypes map[string]bool

	// Recorder + fault injection + id generator.
	calls    map[string]int
	inputs   map[string][]any
	failures map[string][]error
	counter  int

	// Scripted NAT gateway state (createNATGateway poll loop). natScript is the
	// one-shot directive consumed by the next CreateNatGateway; natPending holds
	// each scripted gateway's remaining pending observations and natFinal the
	// state it flips to once they are exhausted.
	natScript  *natScript
	natPending map[string]int
	natFinal   map[string]ec2types.NatGatewayState

	// Scripted teardown transitions (delete poll loops). natDeleteScript is a
	// one-shot directive for the next DeleteNatGateway; natDeleting/eniDraining
	// hold, per resource id, the remaining describe observations that report the
	// resource still tearing down before it is removed from the store.
	natDeleteScript *int
	natDeleting     map[string]int
	eniDraining     map[string]int
}

// natScript is a one-shot directive for the next CreateNatGateway.
type natScript struct {
	pendingPolls int
	final        ec2types.NatGatewayState
}

func newStore() *Store {
	s := &Store{
		Vpcs:                map[string]*ec2types.Vpc{},
		Subnets:             map[string]*ec2types.Subnet{},
		InternetGateways:    map[string]*ec2types.InternetGateway{},
		RouteTables:         map[string]*ec2types.RouteTable{},
		SecurityGroups:      map[string]*ec2types.SecurityGroup{},
		Instances:           map[string]*ec2types.Instance{},
		NatGateways:         map[string]*ec2types.NatGateway{},
		Addresses:           map[string]*ec2types.Address{},
		NetworkInterfaces:   map[string]*ec2types.NetworkInterface{},
		LoadBalancers:       map[string]*elbv2types.LoadBalancer{},
		TargetGroups:        map[string]*elbv2types.TargetGroup{},
		Listeners:           map[string]*elbv2types.Listener{},
		RegisteredTargets:   map[string][]elbv2types.TargetDescription{},
		Parameters:          map[string]string{},
		InstanceTypes:       map[string][]ec2types.ArchitectureType{},
		instanceTypeArchs:   map[string][]ec2types.ArchitectureType{},
		absentInstanceTypes: map[string]bool{},
		calls:               map[string]int{},
		inputs:              map[string][]any{},
		failures:            map[string][]error{},
		natPending:          map[string]int{},
		natFinal:            map[string]ec2types.NatGatewayState{},
		natDeleting:         map[string]int{},
		eniDraining:         map[string]int{},
	}
	s.seed()
	return s
}

// defaultAMI is returned by SSM GetParameter for unseeded parameter names and
// is also seeded into the image catalog so downstream DescribeImages lookups
// (describeImageArch/describeImageRootDevice) resolve.
const defaultAMI = "ami-fake0000"

func (s *Store) seed() {
	// Image catalog: a generic default plus two Ubuntu images with distinct
	// creation dates so setAMI's latest-selection is exercised.
	s.Images = []ec2types.Image{
		{
			ImageId:        aws.String(defaultAMI),
			Name:           aws.String("holodeck-fake-default"),
			CreationDate:   aws.String("2022-01-01T00:00:00.000Z"),
			Architecture:   ec2types.ArchitectureValuesX8664,
			RootDeviceName: aws.String("/dev/xvda"),
			State:          ec2types.ImageStateAvailable,
			OwnerId:        aws.String("099720109477"),
		},
		{
			ImageId:        aws.String("ami-fake0001"),
			Name:           aws.String("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20230101"),
			CreationDate:   aws.String("2023-01-01T00:00:00.000Z"),
			Architecture:   ec2types.ArchitectureValuesX8664,
			RootDeviceName: aws.String("/dev/sda1"),
			State:          ec2types.ImageStateAvailable,
			OwnerId:        aws.String("099720109477"),
		},
		{
			ImageId:        aws.String("ami-fake0002"),
			Name:           aws.String("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20240601"),
			CreationDate:   aws.String("2024-06-01T00:00:00.000Z"),
			Architecture:   ec2types.ArchitectureValuesX8664,
			RootDeviceName: aws.String("/dev/sda1"),
			State:          ec2types.ImageStateAvailable,
			OwnerId:        aws.String("099720109477"),
		},
	}

	// Instance-type catalog covering the types used across tests/data configs so
	// checkInstanceTypes (which lists all types, no filter) validates against the
	// fake. Additional types are echoed permissively by DescribeInstanceTypes
	// when queried by explicit InstanceTypes filter.
	for _, t := range []string{
		"t3.medium", "t3.large", "t3.xlarge",
		"m4.xlarge", "m5.xlarge", "c5.xlarge",
		"g4dn.xlarge", "g5.xlarge", "g5g.xlarge",
	} {
		s.InstanceTypes[t] = archsFor(t)
	}
}

// nextID returns a unique, deterministic id for the given prefix
// (e.g. nextID("vpc") -> "vpc-fake0001"). Callers must hold mu.
func (s *Store) nextID(prefix string) string {
	s.counter++
	return fmt.Sprintf("%s-fake%04d", prefix, s.counter)
}

// record increments the call counter for method and, when a method passes its
// params, appends them to the per-method input log observable via Inputs.
// Every fake client method records its params. Callers must hold mu.
func (s *Store) record(method string, input ...any) {
	s.calls[method]++
	if len(input) > 0 {
		s.inputs[method] = append(s.inputs[method], input[0])
	}
}

// Inputs returns, in call order, the input parameters recorded for method.
// Every fake client method records its params, so this is the observable
// equivalent of the per-method capture closures the provider unit tests
// previously used. Each element is the concrete *ec2.XxxInput /
// *elasticloadbalancingv2.XxxInput / *ssm.XxxInput passed by the caller; it is
// nil if method was never called.
func (s *Store) Inputs(method string) []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.inputs[method]) == 0 {
		return nil
	}
	return append([]any(nil), s.inputs[method]...)
}

// SeedNextNatGatewayState scripts the state reported by the next NAT gateway
// created via CreateNatGateway: it is created (and reported by
// DescribeNatGateways) as pending for the first pendingPolls observations, then
// flips to final. With pendingPolls==0 the gateway is created directly in final.
// The directive is one-shot; without it NAT gateways are created available.
// It exists to drive createNATGateway's poll loop (wait-for-available and
// fail-on-failed) which a single immediate-available response cannot exercise.
func (s *Store) SeedNextNatGatewayState(pendingPolls int, final ec2types.NatGatewayState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.natScript = &natScript{pendingPolls: pendingPolls, final: final}
}

// SeedNextNatGatewayDeleteState scripts the next DeleteNatGateway so the gateway
// lingers in "deleting" for deletingPolls DescribeNatGateways observations
// before it is removed (subsequent describes report it gone), driving
// deleteNATGateway's wait-for-deleted poll loop. Without it, DeleteNatGateway
// removes the gateway immediately.
func (s *Store) SeedNextNatGatewayDeleteState(deletingPolls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	polls := deletingPolls
	s.natDeleteScript = &polls
}

// SeedDrainingENI adds an in-use network interface to vpcID that
// DescribeNetworkInterfaces reports as in-use (blocking) for the first
// blockingPolls observations, then removes — driving waitForENIsDrained's
// re-poll loop. It returns the interface id.
func (s *Store) SeedDrainingENI(vpcID string, blockingPolls int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("eni")
	s.NetworkInterfaces[id] = &ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String(id),
		VpcId:              aws.String(vpcID),
		Status:             ec2types.NetworkInterfaceStatusInUse,
	}
	s.eniDraining[id] = blockingPolls
	return id
}

// failure pops and returns the next injected error for method (FIFO), or nil.
// Callers must hold mu.
func (s *Store) failure(method string) error {
	q := s.failures[method]
	if len(q) == 0 {
		return nil
	}
	err := q[0]
	s.failures[method] = q[1:]
	return err
}

// FailNext queues a one-shot error for the next call to method. Multiple calls
// queue FIFO. The method name is the EC2/ELBv2/SSM API name (e.g. "CreateSubnet").
func (s *Store) FailNext(method string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[method] = append(s.failures[method], err)
}

// CallsTo returns how many times method has been invoked on the fake.
func (s *Store) CallsTo(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[method]
}

// SeedImage adds an image to the catalog returned by DescribeImages.
func (s *Store) SeedImage(img ec2types.Image) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Images = append(s.Images, img)
}

// SeedParameter seeds an SSM parameter value returned by GetParameter.
func (s *Store) SeedParameter(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Parameters[name] = value
}

// SeedInstanceType adds an instance type to the no-filter catalog returned by
// DescribeInstanceTypes (architecture inferred from the type prefix).
func (s *Store) SeedInstanceType(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InstanceTypes[name] = archsFor(name)
}

// SeedInstanceTypeArchs registers explicit supported architectures for an
// instance type, overriding the prefix heuristic in DescribeInstanceTypes'
// filtered (InstanceTypes) responses. It models types whose architecture the
// heuristic cannot infer, e.g. mac variants or multi-architecture types.
func (s *Store) SeedInstanceTypeArchs(name string, archs ...ec2types.ArchitectureType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instanceTypeArchs[name] = archs
}

// SeedInstanceTypeAbsent marks an instance type as not offered, so a filtered
// DescribeInstanceTypes query for it returns no results — the
// region-unavailable case the provider treats as "instance type not found".
func (s *Store) SeedInstanceTypeAbsent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.absentInstanceTypes[name] = true
}

// SetImages replaces the DescribeImages catalog (clearing the seeded default
// images) so a test can present exactly the AMIs a resolution path should see.
func (s *Store) SetImages(imgs ...ec2types.Image) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Images = append([]ec2types.Image(nil), imgs...)
}

// SetInstanceTypeCatalog replaces the no-filter DescribeInstanceTypes catalog
// (the types checkInstanceTypes scans) with exactly the given types, clearing
// the seeded defaults. Passing none yields an empty catalog — the
// region-unavailable case that makes checkInstanceTypes reject its type.
func (s *Store) SetInstanceTypeCatalog(names ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InstanceTypes = map[string][]ec2types.ArchitectureType{}
	for _, n := range names {
		s.InstanceTypes[n] = archsFor(n)
	}
}

// ResourceCounts returns the number of live (non-terminated/non-deleted)
// resources per kind. Seed data (images, instance types, SSM parameters) is
// excluded. Keys are stable and consumed by the E2E suite.
func (s *Store) ResourceCounts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countsLocked()
}

// Empty reports whether all live resource counts are zero.
func (s *Store) Empty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.countsLocked() {
		if v > 0 {
			return false
		}
	}
	return true
}

func (s *Store) countsLocked() map[string]int {
	liveInstances := 0
	for _, inst := range s.Instances {
		if inst.State == nil || inst.State.Name != ec2types.InstanceStateNameTerminated {
			liveInstances++
		}
	}
	return map[string]int{
		"vpcs":              len(s.Vpcs),
		"subnets":           len(s.Subnets),
		"internetgateways":  len(s.InternetGateways),
		"routetables":       len(s.RouteTables),
		"securitygroups":    len(s.SecurityGroups),
		"instances":         liveInstances,
		"natgateways":       len(s.NatGateways),
		"addresses":         len(s.Addresses),
		"networkinterfaces": len(s.NetworkInterfaces),
		"loadbalancers":     len(s.LoadBalancers),
		"targetgroups":      len(s.TargetGroups),
		"listeners":         len(s.Listeners),
	}
}

// archsFor returns the supported architectures for an instance type, matching
// the provider's inferArchFromInstanceType heuristic: arm64-only families
// (g5g/c6g/m6g/t4g) report arm64, everything else reports x86_64.
func archsFor(instanceType string) []ec2types.ArchitectureType {
	for _, p := range []string{"g5g", "c6g", "m6g", "t4g", "c7g", "m7g", "r6g", "r7g"} {
		if strings.HasPrefix(instanceType, p) {
			return []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64}
		}
	}
	return []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664}
}

// tagsFromSpecs flattens TagSpecifications into a single tag slice.
func tagsFromSpecs(specs []ec2types.TagSpecification) []ec2types.Tag {
	var tags []ec2types.Tag
	for _, spec := range specs {
		tags = append(tags, spec.Tags...)
	}
	return tags
}

// notFound builds an error whose text contains the exact AWS error-code
// substring the provider's idempotent-delete branches match on.
func notFound(code, id string) error {
	return fmt.Errorf("%s: resource %s not found", code, id)
}
