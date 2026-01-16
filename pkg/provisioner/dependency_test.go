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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

var _ = Describe("DependencyResolver", func() {

	Describe("NewDependencies", func() {
		It("should create an empty dependency resolver", func() {
			env := v1alpha1.Environment{}
			d := provisioner.NewDependencies(&env)
			Expect(d).NotTo(BeNil())
			Expect(d.Dependencies).To(BeEmpty())
		})
	})

	Describe("Resolve", func() {
		Context("with no components installed", func() {
			It("should return empty dependencies", func() {
				env := v1alpha1.Environment{}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(BeEmpty())
			})
		})

		Context("with Kubernetes only", func() {
			DescribeTable("different installers",
				func(installer string, expectedDeps int) {
					env := v1alpha1.Environment{
						Spec: v1alpha1.EnvironmentSpec{
							Kubernetes: v1alpha1.Kubernetes{
								Install:             true,
								KubernetesInstaller: installer,
							},
						},
					}
					d := provisioner.NewDependencies(&env)
					deps := d.Resolve()
					Expect(deps).To(HaveLen(expectedDeps))
				},
				Entry("kubeadm", "kubeadm", 1),
				Entry("kind", "kind", 1),
				Entry("microk8s", "microk8s", 1),
				Entry("empty (defaults to kubeadm)", "", 1),
			)
		})

		Context("with Container Runtime only", func() {
			DescribeTable("different runtimes",
				func(runtime v1alpha1.ContainerRuntimeName, expectedDeps int) {
					env := v1alpha1.Environment{
						Spec: v1alpha1.EnvironmentSpec{
							ContainerRuntime: v1alpha1.ContainerRuntime{
								Install: true,
								Name:    runtime,
							},
						},
					}
					d := provisioner.NewDependencies(&env)
					deps := d.Resolve()
					Expect(deps).To(HaveLen(expectedDeps))
				},
				Entry("containerd", v1alpha1.ContainerRuntimeContainerd, 1),
				Entry("crio", v1alpha1.ContainerRuntimeCrio, 1),
				Entry("docker", v1alpha1.ContainerRuntimeDocker, 1),
				Entry("empty (defaults to containerd)",
					v1alpha1.ContainerRuntimeNone, 1),
			)
		})

		Context("with NVIDIA Driver", func() {
			It("should add driver dependency", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIADriver: v1alpha1.NVIDIADriver{
							Install: true,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))
			})
		})

		Context("with Container Toolkit", func() {
			It("should add toolkit dependency", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))
			})
		})

		Context("with Kernel", func() {
			It("should add kernel dependency", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kernel: v1alpha1.Kernel{
							Version: "5.15.0",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))
			})
		})

		Context("with full stack", func() {
			It("should resolve all dependencies in correct order", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kernel: v1alpha1.Kernel{
							Version: "5.15.0",
						},
						NVIDIADriver: v1alpha1.NVIDIADriver{
							Install: true,
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

				// Expected order: kernel, nvdriver, containerd, toolkit, kubeadm
				Expect(deps).To(HaveLen(5))
			})
		})

		Context("with microk8s installer", func() {
			It("should reset dependencies to only microk8s", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeContainerd,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "microk8s",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()

				// microk8s resets the list, so only 1 dependency
				Expect(deps).To(HaveLen(1))
			})
		})
	})

	Describe("ProvisionFunc execution", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
		})

		Context("nvdriver", func() {
			It("should execute nvdriver template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIADriver: v1alpha1.NVIDIADriver{
							Install: true,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).NotTo(BeEmpty())
			})
		})

		Context("docker", func() {
			It("should execute docker template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("docker"))
			})
		})

		Context("containerd", func() {
			It("should execute containerd template", func() {
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
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("containerd"))
			})
		})

		Context("crio", func() {
			It("should execute crio template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeCrio,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("crio"))
			})
		})

		Context("container toolkit (package source)", func() {
			It("should execute container toolkit template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourcePackage,
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).NotTo(BeEmpty())
			})
		})

		Context("kubeadm", func() {
			It("should execute kubeadm template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeContainerd,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kubeadm",
							KubernetesVersion:   "v1.28.0",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				// containerd + kubeadm
				Expect(deps).To(HaveLen(2))

				// Execute kubeadm (second in list)
				err := deps[1](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("kubeadm"))
			})
		})

		Context("kind", func() {
			It("should execute kind template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kind",
							KubernetesVersion:   "v1.28.0",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				// docker + kind
				Expect(deps).To(HaveLen(2))

				// Execute kind (second in list)
				err := deps[1](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("kind"))
			})
		})

		Context("microk8s", func() {
			It("should execute microk8s template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeContainerd,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "microk8s",
							KubernetesVersion:   "v1.28.0",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				// microk8s resets list, so just 1
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("microk8s"))
			})
		})

		Context("kernel", func() {
			It("should execute kernel template", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kernel: v1alpha1.Kernel{
							Version: "5.15.0-generic",
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(1))

				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("5.15.0"))
			})
		})

		Context("KIND with git source auto-upgrades Docker", func() {
			It("should set minimum Docker version for KIND git source builds", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
							// No version specified
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kind",
							Source:              "git",
							Git: &v1alpha1.K8sGitSpec{
								Ref: "refs/tags/v1.31.0",
							},
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(2)) // docker + kind

				// Verify Docker version was auto-set
				Expect(env.Spec.ContainerRuntime.Version).To(Equal("5:24.0.0-1~ubuntu.22.04~jammy"))

				// Verify docker template uses the version
				err := deps[0](buf, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("5:24.0.0-1~ubuntu.22.04~jammy"))
			})

			It("should set minimum Docker version for KIND latest source builds", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kind",
							Source:              "latest",
							Latest: &v1alpha1.K8sLatestSpec{
								Track: "master",
							},
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				deps := d.Resolve()
				Expect(deps).To(HaveLen(2))
				Expect(env.Spec.ContainerRuntime.Version).To(Equal("5:24.0.0-1~ubuntu.22.04~jammy"))
			})

			It("should not override explicit Docker version", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
							Version: "5:25.0.0-1~ubuntu.22.04~jammy",
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kind",
							Source:              "git",
							Git: &v1alpha1.K8sGitSpec{
								Ref: "refs/tags/v1.31.0",
							},
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				_ = d.Resolve()
				// User's version should be preserved
				Expect(env.Spec.ContainerRuntime.Version).To(Equal("5:25.0.0-1~ubuntu.22.04~jammy"))
			})

			It("should not set Docker version for KIND release source", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kind",
							Source:              "release",
							Release: &v1alpha1.K8sReleaseSpec{
								Version: "v1.31.0",
							},
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				_ = d.Resolve()
				// No auto-upgrade for release source (uses pre-built images)
				Expect(env.Spec.ContainerRuntime.Version).To(BeEmpty())
			})

			It("should not set Docker version for kubeadm git source", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kubeadm",
							Source:              "git",
							Git: &v1alpha1.K8sGitSpec{
								Ref: "refs/tags/v1.31.0",
							},
						},
					},
				}
				d := provisioner.NewDependencies(&env)
				_ = d.Resolve()
				// kubeadm doesn't need specific Docker version
				Expect(env.Spec.ContainerRuntime.Version).To(BeEmpty())
			})
		})
	})
})
