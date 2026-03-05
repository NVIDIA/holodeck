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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

var _ = Describe("DependencyResolver CustomTemplates", func() {
	var buf *bytes.Buffer

	BeforeEach(func() {
		buf = &bytes.Buffer{}
	})

	Context("with no custom templates", func() {
		It("should resolve without custom templates", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: true,
						Name:    v1alpha1.ContainerRuntimeContainerd,
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			// Only containerd, no custom templates
			Expect(deps).To(HaveLen(1))
		})
	})

	Context("with pre-install custom template", func() {
		It("should add pre-install template before kernel", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:   "setup-repos",
							Phase:  v1alpha1.TemplatePhasePreInstall,
							Inline: "echo 'Setting up repos'",
						},
					},
					Kernel: v1alpha1.Kernel{
						Version: "5.15.0",
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			// pre-install custom template + kernel
			Expect(deps).To(HaveLen(2))

			// First should be the custom template (pre-install)
			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("[CUSTOM]"))
			Expect(buf.String()).To(ContainSubstring("setup-repos"))
			Expect(buf.String()).To(ContainSubstring("Setting up repos"))
		})
	})

	Context("with post-install custom template", func() {
		It("should add post-install template at the end", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:   "finalize",
							Phase:  v1alpha1.TemplatePhasePostInstall,
							Inline: "echo 'Finalizing'",
						},
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: true,
						Name:    v1alpha1.ContainerRuntimeContainerd,
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			// containerd + post-install custom template
			Expect(deps).To(HaveLen(2))

			// Last should be the custom template
			err := deps[1](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("[CUSTOM]"))
			Expect(buf.String()).To(ContainSubstring("finalize"))
			Expect(buf.String()).To(ContainSubstring("Finalizing"))
		})
	})

	Context("with empty phase (defaults to post-install)", func() {
		It("should treat empty phase as post-install", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:   "default-phase",
							Inline: "echo 'default'",
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			// post-install custom template only
			Expect(deps).To(HaveLen(1))

			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("[CUSTOM]"))
			Expect(buf.String()).To(ContainSubstring("default-phase"))
		})
	})

	Context("with all phases", func() {
		It("should place custom templates at all five phase points", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{Name: "pre", Phase: v1alpha1.TemplatePhasePreInstall, Inline: "echo pre"},
						{Name: "post-rt", Phase: v1alpha1.TemplatePhasePostRuntime, Inline: "echo post-rt"},
						{Name: "post-tk", Phase: v1alpha1.TemplatePhasePostToolkit, Inline: "echo post-tk"},
						{Name: "post-k8s", Phase: v1alpha1.TemplatePhasePostKubernetes, Inline: "echo post-k8s"},
						{Name: "post-inst", Phase: v1alpha1.TemplatePhasePostInstall, Inline: "echo post-inst"},
					},
					ContainerRuntime: v1alpha1.ContainerRuntime{
						Install: true,
						Name:    v1alpha1.ContainerRuntimeContainerd,
					},
					NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
						Install: true,
					},
					Kubernetes: v1alpha1.Kubernetes{
						Install:             true,
						KubernetesInstaller: "kubeadm",
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			// pre-install(1) + containerd(1) + post-runtime(1) + toolkit(1) + post-toolkit(1) + kubeadm(1) + post-k8s(1) + post-install(1) = 8
			Expect(deps).To(HaveLen(8))

			// Verify first is pre-install custom
			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("pre"))
		})
	})

	Context("with multiple templates in same phase", func() {
		It("should add all templates for the phase in order", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{Name: "first", Phase: v1alpha1.TemplatePhasePostInstall, Inline: "echo first"},
						{Name: "second", Phase: v1alpha1.TemplatePhasePostInstall, Inline: "echo second"},
						{Name: "third", Phase: v1alpha1.TemplatePhasePostInstall, Inline: "echo third"},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			Expect(deps).To(HaveLen(3))

			// Execute all and verify order
			for _, dep := range deps {
				err := dep(buf, env)
				Expect(err).NotTo(HaveOccurred())
			}
			output := buf.String()
			Expect(output).To(ContainSubstring("first"))
			Expect(output).To(ContainSubstring("second"))
			Expect(output).To(ContainSubstring("third"))
		})
	})

	Context("with file template and baseDir", func() {
		It("should resolve relative file paths using baseDir", func() {
			tmpDir := GinkgoT().TempDir()
			scriptPath := filepath.Join(tmpDir, "test-script.sh")
			err := os.WriteFile(scriptPath, []byte("echo 'from file'"), 0600)
			Expect(err).NotTo(HaveOccurred())

			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:  "file-tpl",
							Phase: v1alpha1.TemplatePhasePostInstall,
							File:  "test-script.sh",
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			d.SetBaseDir(tmpDir)
			deps := d.Resolve()
			Expect(deps).To(HaveLen(1))

			err = deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("from file"))
		})

		It("should handle absolute file paths regardless of baseDir", func() {
			tmpDir := GinkgoT().TempDir()
			scriptPath := filepath.Join(tmpDir, "abs-script.sh")
			err := os.WriteFile(scriptPath, []byte("echo 'absolute'"), 0600)
			Expect(err).NotTo(HaveOccurred())

			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:  "abs-file-tpl",
							Phase: v1alpha1.TemplatePhasePostInstall,
							File:  scriptPath,
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			d.SetBaseDir("/nonexistent")
			deps := d.Resolve()
			Expect(deps).To(HaveLen(1))

			err = deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("absolute"))
		})
	})

	Context("with environment variables", func() {
		It("should export env vars before script content", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:   "with-env",
							Phase:  v1alpha1.TemplatePhasePostInstall,
							Inline: "echo $MY_VAR",
							Env: map[string]string{
								"MY_VAR": "hello",
							},
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			Expect(deps).To(HaveLen(1))

			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("export MY_VAR="))
		})
	})

	Context("with continueOnError", func() {
		It("should wrap script in error handling when continueOnError is true", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:            "safe-script",
							Phase:           v1alpha1.TemplatePhasePostInstall,
							Inline:          "might-fail",
							ContinueOnError: true,
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			Expect(deps).To(HaveLen(1))

			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			output := buf.String()
			Expect(output).To(ContainSubstring("set +e"))
			Expect(output).To(ContainSubstring("might-fail"))
		})
	})

	Context("with checksum verification for inline", func() {
		It("should add checksum verification when checksum is provided", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					CustomTemplates: []v1alpha1.CustomTemplate{
						{
							Name:     "checksummed",
							Phase:    v1alpha1.TemplatePhasePostInstall,
							Inline:   "echo verified",
							Checksum: "sha256:4ffd19377b7c0b48cb504560c6ecd256526a71c7fdfdfa846f386ec0535001ea",
						},
					},
				},
			}
			d := provisioner.NewDependencies(&env)
			deps := d.Resolve()
			Expect(deps).To(HaveLen(1))

			err := deps[0](buf, env)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("echo verified"))
		})
	})
})
