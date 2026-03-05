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

package provisioner_test

import (
	"bytes"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

func testDataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "tests", "data", name)
}

var _ = Describe("Custom Templates Integration", func() {

	Describe("YAML parsing", func() {
		It("should parse custom templates from YAML fixture", func() {
			env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](
				testDataPath("test_custom_templates.yaml"),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Spec.CustomTemplates).To(HaveLen(4))

			// Verify first template
			Expect(env.Spec.CustomTemplates[0].Name).To(Equal("configure-repos"))
			Expect(env.Spec.CustomTemplates[0].Phase).To(Equal(v1alpha1.TemplatePhasePreInstall))
			Expect(env.Spec.CustomTemplates[0].Inline).NotTo(BeEmpty())

			// Verify template with env vars
			Expect(env.Spec.CustomTemplates[2].Name).To(Equal("install-monitoring"))
			Expect(env.Spec.CustomTemplates[2].Env).To(HaveKeyWithValue("MONITORING_NS", "monitoring"))
			Expect(env.Spec.CustomTemplates[2].Env).To(HaveKeyWithValue("PROMETHEUS_VERSION", "2.45.0"))

			// Verify template with continueOnError
			Expect(env.Spec.CustomTemplates[3].ContinueOnError).To(BeTrue())
			Expect(env.Spec.CustomTemplates[3].Timeout).To(Equal(300))
		})
	})

	Describe("Dependency resolution order", func() {
		It("should place custom templates at correct positions in dependency list", func() {
			env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](
				testDataPath("test_custom_templates.yaml"),
			)
			Expect(err).NotTo(HaveOccurred())

			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()

			// Expected order:
			// 1. pre-install custom template (configure-repos)
			// 2. kernel (not in YAML, so skipped)
			// 3. nvdriver
			// 4. containerd
			// 5. post-runtime custom template (post-runtime-config)
			// 6. container toolkit
			// 7. kubeadm
			// 8. post-kubernetes custom template (install-monitoring)
			// 9. post-install custom template (cleanup)
			Expect(deps).To(HaveLen(8))

			// Verify first is pre-install custom template
			var buf bytes.Buffer
			err = deps[0](&buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("[CUSTOM]"))
			Expect(buf.String()).To(ContainSubstring("configure-repos"))

			// Verify last is post-install custom template
			buf.Reset()
			err = deps[7](&buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("[CUSTOM]"))
			Expect(buf.String()).To(ContainSubstring("cleanup"))
			Expect(buf.String()).To(ContainSubstring("set +e"))
		})
	})
})
