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

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/aws/awsfake"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/tests/common"
)

// newMockProvider builds a Provider wired to a fresh in-memory awsfake so the
// real Create/Delete/DryRun code runs credential-free against an observable
// resource store. It returns the provider, the fake (for resource-count
// assertions and fault injection), the loaded config, and the cache-file path.
func newMockProvider(cfgFile string) (provider.Provider, *awsfake.Fake, *v1alpha1.Environment, string) {
	GinkgoHelper()

	log := logger.NewLogger()
	log.Out = GinkgoWriter

	fake := awsfake.New()

	cfgPath := filepath.Join(packagePath, "data", cfgFile)
	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cfgPath)
	Expect(err).NotTo(HaveOccurred(), "failed to read config %s", cfgPath)
	cfg.Name += "-" + common.GenerateUID()

	cacheFile := filepath.Join(GinkgoT().TempDir(), "cache.yaml")

	p, err := newProvider(log, cfg, cacheFile,
		aws.WithEC2Client(fake.EC2),
		aws.WithSSMClient(fake.SSM),
		aws.WithELBv2Client(fake.ELBv2),
		aws.WithSleep(func(time.Duration) {}))
	Expect(err).NotTo(HaveOccurred(), "failed to build mock provider")

	return p, fake, &cfg, cacheFile
}

// AWS Mock E2E drives the real AWS provider lifecycle against the stateful
// in-memory fake. Every spec is credential-free (no AWS_* / E2E_SSH_KEY) and
// carries Label("mock") so CI can select it without touching real AWS.
var _ = Describe("AWS Mock E2E", Label("mock"), func() {
	DescribeTable("provisions and tears down topologies against the fake",
		func(cfgFile string, wantInstances int, wantNLB bool, wantSubnets int, wantRouteTables int, wantSecurityGroups int) {
			p, fake, _, cacheFile := newMockProvider(cfgFile)

			Expect(p.DryRun()).To(Succeed())
			Expect(p.Create()).To(Succeed())

			counts := fake.Store.ResourceCounts()
			Expect(counts["instances"]).To(Equal(wantInstances),
				"unexpected live instance count: %v", counts)
			Expect(counts["vpcs"]).To(Equal(1), "expected exactly one VPC: %v", counts)
			Expect(counts["subnets"]).To(Equal(wantSubnets),
				"unexpected subnet count: %v", counts)
			Expect(counts["routetables"]).To(Equal(wantRouteTables),
				"unexpected route table count: %v", counts)
			Expect(counts["securitygroups"]).To(Equal(wantSecurityGroups),
				"unexpected security group count: %v", counts)
			Expect(counts["networkinterfaces"]).To(Equal(wantInstances),
				"expected one network interface per instance: %v", counts)
			if wantNLB {
				Expect(counts["loadbalancers"]).To(Equal(1),
					"HA cluster must provision an NLB: %v", counts)
			} else {
				Expect(counts["loadbalancers"]).To(Equal(0),
					"non-HA topology must not provision an NLB: %v", counts)
			}

			_, err := os.Stat(cacheFile)
			Expect(err).NotTo(HaveOccurred(), "Create must write the cache file")

			Expect(p.Delete()).To(Succeed())
			Expect(fake.Store.Empty()).To(BeTrue(),
				"orphaned resources after delete: %v", fake.Store.ResourceCounts())
		},
		// Cluster topologies share one route table: CreateCluster only calls
		// createPublicRouteTable (cluster.go Phase 1b) — the private route
		// table / NAT gateway path (createPrivateRouteTable, createNATGateway)
		// is defined but never invoked, since cluster instances sit in the
		// public subnet with a direct IGW route (see cluster.go:138-143).
		Entry("single node", "test_aws.yml", 1, false, 1, 1, 1),
		Entry("minimal cluster (1 CP + 1 worker)", "test_cluster_minimal.yaml", 2, false, 2, 1, 2),
		Entry("dedicated CP cluster (1 CP + 3 GPU)", "test_cluster_1cp_3gpu.yaml", 4, false, 2, 1, 2),
		Entry("HA cluster (3 CP + 2 workers)", "test_cluster_ha_3cp_2gpu.yaml", 5, true, 2, 1, 2),
	)

	DescribeTable("single-node create rolls back cleanly on injected API failure",
		func(failMethod string) {
			p, fake, _, _ := newMockProvider("test_aws.yml")
			fake.Store.FailNext(failMethod, fmt.Errorf("injected: %s down", failMethod))

			Expect(p.Create()).NotTo(Succeed())
			Expect(fake.Store.Empty()).To(BeTrue(),
				"cleanupStack left orphans after %s failure: %v", failMethod, fake.Store.ResourceCounts())
		},
		Entry("at CreateSubnet", "CreateSubnet"),
		Entry("at CreateInternetGateway", "CreateInternetGateway"),
		Entry("at CreateRouteTable", "CreateRouteTable"),
		Entry("at CreateSecurityGroup", "CreateSecurityGroup"),
		Entry("at RunInstances", "RunInstances"),
	)

	It("cluster partial failure is fully cleaned by a subsequent Delete", func() {
		p, fake, _, _ := newMockProvider("test_cluster_minimal.yaml")
		fake.Store.FailNext("RunInstances", fmt.Errorf("injected capacity error"))

		// cluster.go has no in-flight rollback stack, so a mid-create failure
		// leaves partial resources behind (documented behavior).
		Expect(p.Create()).NotTo(Succeed())
		Expect(fake.Store.Empty()).To(BeFalse(),
			"partial cluster resources should remain before Delete")

		Expect(p.Delete()).To(Succeed())
		Expect(fake.Store.Empty()).To(BeTrue(),
			"Delete must reap partially-created cluster: %v", fake.Store.ResourceCounts())
	})

	It("Delete is idempotent", func() {
		p, fake, _, _ := newMockProvider("test_aws.yml")
		Expect(p.Create()).To(Succeed())
		Expect(p.Delete()).To(Succeed())
		Expect(p.Delete()).To(Succeed(), "second delete must tolerate NotFound errors")
		Expect(fake.Store.Empty()).To(BeTrue())
	})
})
