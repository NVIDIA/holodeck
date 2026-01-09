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

package list_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/cmd/cli/list"
	"github.com/NVIDIA/holodeck/internal/logger"
)

func TestList(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "List Command Suite")
}

var _ = Describe("List Command", func() {
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
			cmd := list.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("list"))
			Expect(cmd.Aliases).To(ContainElement("ls"))
			Expect(cmd.Usage).To(ContainSubstring("List"))
		})

		It("should have cachepath flag", func() {
			cmd := list.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("cachepath"))
			Expect(flagNames).To(HaveKey("c"))
		})

		It("should have quiet flag", func() {
			cmd := list.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}
			Expect(flagNames).To(HaveKey("quiet"))
			Expect(flagNames).To(HaveKey("q"))
		})
	})

	Describe("Command action", func() {
		It("should have an action", func() {
			cmd := list.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})
	})
})
