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

package provisioner

import (
	"bytes"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/pkg/provisioner/templates"
)

var _ = Describe("Provisioner", func() {

	Describe("Shebang constant", func() {
		It("should start with shebang", func() {
			Expect(Shebang).To(HavePrefix("#!"))
		})

		It("should use bash", func() {
			Expect(Shebang).To(ContainSubstring("bash"))
		})

		It("should set exit on error", func() {
			Expect(Shebang).To(ContainSubstring("set -xe"))
		})
	})

	Describe("remoteKindConfig constant", func() {
		It("should point to kubernetes config directory", func() {
			Expect(remoteKindConfig).To(Equal("/etc/kubernetes/kind.yaml"))
		})
	})

	Describe("addScriptHeader", func() {
		var buf bytes.Buffer

		BeforeEach(func() {
			buf.Reset()
		})

		It("should add shebang to empty buffer", func() {
			err := addScriptHeader(&buf)
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			Expect(output).To(HavePrefix("#!"))
			Expect(output).To(ContainSubstring("bash"))
		})

		It("should add common functions", func() {
			err := addScriptHeader(&buf)
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			// Check for key elements from common functions
			Expect(output).To(ContainSubstring("holodeck"))
		})

		It("should produce valid bash script header", func() {
			err := addScriptHeader(&buf)
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			lines := strings.Split(output, "\n")
			Expect(len(lines)).To(BeNumerically(">", 1))
			Expect(lines[0]).To(HavePrefix("#!"))
		})

		It("should include set -xe for safety", func() {
			err := addScriptHeader(&buf)
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			Expect(output).To(ContainSubstring("set -xe"))
		})
	})

	Describe("CommonFunctions template", func() {
		It("should not be empty", func() {
			Expect(templates.CommonFunctions).NotTo(BeEmpty())
		})

		It("should contain logging functions", func() {
			Expect(templates.CommonFunctions).To(ContainSubstring("holodeck_log"))
		})

		It("should contain retry logic", func() {
			Expect(templates.CommonFunctions).To(ContainSubstring("holodeck_retry"))
		})

		It("should contain progress tracking", func() {
			Expect(templates.CommonFunctions).To(ContainSubstring("holodeck_progress"))
		})

		It("should contain verification helpers", func() {
			Expect(templates.CommonFunctions).To(ContainSubstring("holodeck_verify"))
		})

		It("should contain installation marking", func() {
			Expect(templates.CommonFunctions).To(ContainSubstring("holodeck_mark_installed"))
		})
	})

	Describe("Provisioner struct", func() {
		It("should have all required fields", func() {
			p := Provisioner{
				HostUrl:  "test-host",
				UserName: "test-user",
				KeyPath:  "/path/to/key",
			}

			Expect(p.HostUrl).To(Equal("test-host"))
			Expect(p.UserName).To(Equal("test-user"))
			Expect(p.KeyPath).To(Equal("/path/to/key"))
		})
	})
})
