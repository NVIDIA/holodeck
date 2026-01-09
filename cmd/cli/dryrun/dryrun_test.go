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

package dryrun_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/cmd/cli/dryrun"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestDryrun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dryrun Command Suite")
}

var _ = Describe("Dryrun Command", func() {
	var (
		log *logger.FunLogger
		buf bytes.Buffer
	)

	BeforeEach(func() {
		log = logger.NewLogger()
		log.Out = &buf
	})

	Describe("NewCommand", func() {
		It("should create a valid command", func() {
			cmd := dryrun.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("dryrun"))
			Expect(cmd.Usage).To(ContainSubstring("dryrun"))
		})

		It("should have envFile flag", func() {
			cmd := dryrun.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("envFile"))
			Expect(flagNames).To(HaveKey("f"))
		})

		It("should have an action", func() {
			cmd := dryrun.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should have a before hook", func() {
			cmd := dryrun.NewCommand(log)
			Expect(cmd.Before).NotTo(BeNil())
		})
	})
})
