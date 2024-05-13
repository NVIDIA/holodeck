/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

// Actual test suite
var _ = Describe("Holodeck", func() {
	type options struct {
		cachePath string
		cachefile string

		cfg v1alpha1.Environment
	}
	opts := options{}

	// Create a logger
	log := logger.NewLogger()
	log.CI = true

	Context("When testing an environment", Ordered, func() {
		BeforeAll(func(ctx context.Context) {
			//
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](*EnvFile)
			Expect(err).ToNot(HaveOccurred())

			// set cache path
			if opts.cachePath == "" {
				opts.cachePath = filepath.Join(*LogArtifactDir, "holodeck")
			}
			opts.cachefile = filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")

			opts.cfg.Spec.Provider = v1alpha1.ProviderAWS
		})

		AfterAll(func(ctx context.Context) {
			// remove the cache file if the test is successful
			if !CurrentGinkgoTestDescription().Failed {
				err := os.Remove(opts.cachefile)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		Context("and calling dryrun to validate the file", func() {
			It("validate the provider", func() {
				client, err := aws.New(log, opts.cfg, *EnvFile)
				Expect(err).ToNot(HaveOccurred())

				err = client.DryRun()
				Expect(err).ToNot(HaveOccurred())
			})

			It("validate the provisioner", func() {
				err := provisioner.Dryrun(log, opts.cfg)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("and calling provision to create the environment", func() {
			AfterAll(func(ctx context.Context) {
				// Delete the environment
				client, err := aws.New(log, opts.cfg, opts.cachefile)
				Expect(err).ToNot(HaveOccurred())

				err = client.Delete()
				Expect(err).ToNot(HaveOccurred())
			})
			It("create the environment", func() {
				client, err := aws.New(log, opts.cfg, opts.cachefile)
				Expect(err).ToNot(HaveOccurred())

				err = client.Create()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
