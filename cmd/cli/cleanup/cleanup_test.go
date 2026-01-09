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

package cleanup_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/cmd/cli/cleanup"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestCleanup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cleanup Command Suite")
}

var _ = Describe("Cleanup Command", func() {
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
			cmd := cleanup.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("cleanup"))
			Expect(cmd.Usage).To(ContainSubstring("Clean up"))
		})

		It("should have region flag", func() {
			cmd := cleanup.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("region"))
			Expect(flagNames).To(HaveKey("r"))
		})

		It("should have force flag", func() {
			cmd := cleanup.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("force"))
			Expect(flagNames).To(HaveKey("f"))
		})

		It("should have an action", func() {
			cmd := cleanup.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should have a description", func() {
			cmd := cleanup.NewCommand(log)
			Expect(cmd.Description).NotTo(BeEmpty())
			Expect(cmd.Description).To(ContainSubstring("VPC"))
		})
	})
})
