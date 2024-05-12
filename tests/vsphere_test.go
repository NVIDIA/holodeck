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
	"github.com/NVIDIA/holodeck/pkg/provider"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
	"github.com/NVIDIA/holodeck/tests/common"
)

// Actual test suite
var _ = Describe("VSPHERE", func() {
	type options struct {
		cachePath string
		cachefile string

		cfg v1alpha1.Environment
	}
	opts := options{}

	// Create a logger
	log := logger.NewLogger()

	Context("When testing an VSPHERE environment", Ordered, func() {
		var provider provider.Provider
		var err error

		BeforeAll(func(ctx context.Context) {
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](*EnvFile)
			Expect(err).ToNot(HaveOccurred())

			// Set unique name for the environment
			opts.cfg.Name = opts.cfg.Name + "-" + common.GenerateUID()
			// set cache path
			opts.cachePath = *LogArtifactDir
			// set cache file
			opts.cachefile = filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")
			// Create cachedir directory
			if _, err := os.Stat(opts.cachePath); os.IsNotExist(err) {
				err := os.Mkdir(opts.cachePath, 0755)
				Expect(err).ToNot(HaveOccurred())
			}

			provider, err = newProvider(log, opts.cfg, opts.cachefile)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func(ctx context.Context) {
			// remove the cache file if the test is successful
			if !CurrentSpecReport().Failed() {
				err := os.Remove(opts.cachefile)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		Context("and calling dryrun to validate the file", func() {
			It("validates the provider", func() {
				err = provider.DryRun()
				Expect(err).ToNot(HaveOccurred())
			})

			It("validates the provisioner", func() {
				err := provisioner.Dryrun(log, opts.cfg)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("and calling provision to create the environment", func() {
			AfterAll(func(ctx context.Context) {
				// Delete the environment
				// We delete using AfterAll so that the environment is deleted even if the test fails
				// This is to avoid leaving resources behind, if deletion fails, the test will fail
				// and will report the error
				err = provider.Delete()
				Expect(err).ToNot(HaveOccurred())
			})
			It("Creates the requested environment", func() {
				err = provider.Create()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
