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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

var _ = Describe("DependencyResolver", func() {

	Describe("NewDependencies", func() {
		It("should create an empty dependency resolver", func() {
			env := v1alpha1.Environment{}
			d := provisioner.NewDependencies(env)
			Expect(d).NotTo(BeNil())
			Expect(d.Dependencies).To(BeEmpty())
		})
	})

	Describe("Resolve", func() {
		Context("with no components installed", func() {
			It("should return empty dependencies", func() {
				env := v1alpha1.Environment{}
				d := provisioner.NewDependencies(env)
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
					d := provisioner.NewDependencies(env)
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
					d := provisioner.NewDependencies(env)
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
				d := provisioner.NewDependencies(env)
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
				d := provisioner.NewDependencies(env)
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
				d := provisioner.NewDependencies(env)
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
				d := provisioner.NewDependencies(env)
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
				d := provisioner.NewDependencies(env)
				deps := d.Resolve()

				// microk8s resets the list, so only 1 dependency
				Expect(deps).To(HaveLen(1))
			})
		})
	})
})
