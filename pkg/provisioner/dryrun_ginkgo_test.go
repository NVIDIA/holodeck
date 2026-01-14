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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

var _ = Describe("Dryrun", func() {

	Describe("validateCTKConfig", func() {
		var log *logger.FunLogger

		BeforeEach(func() {
			log = logger.NewLogger()
		})

		Context("with package source", func() {
			It("should succeed with explicit package source", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourcePackage,
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with empty source (defaults to package)", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  "",
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with package version specified", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourcePackage,
							Package: &v1alpha1.CTKPackageSpec{
								Version: "1.17.3-1",
							},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with legacy version field", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourcePackage,
							Version: "1.17.3-1",
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with git source", func() {
			It("should succeed with valid git config", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceGit,
							Git: &v1alpha1.CTKGitSpec{
								Ref: "v1.17.3",
							},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with custom repo", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceGit,
							Git: &v1alpha1.CTKGitSpec{
								Repo: "https://github.com/myorg/toolkit.git",
								Ref:  "main",
							},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail without git config", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceGit,
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("git source requires"))
			})

			It("should fail without ref", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceGit,
							Git:     &v1alpha1.CTKGitSpec{},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ref"))
			})
		})

		Context("with latest source", func() {
			It("should succeed with default config", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceLatest,
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with custom track", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceLatest,
							Latest: &v1alpha1.CTKLatestSpec{
								Track: "release-1.17",
							},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with custom repo", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceLatest,
							Latest: &v1alpha1.CTKLatestSpec{
								Repo:  "https://github.com/myorg/toolkit.git",
								Track: "develop",
							},
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with unknown source", func() {
			It("should fail", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  "unknown",
						},
					},
				}
				err := validateCTKConfig(log, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown CTK source"))
			})
		})
	})

	Describe("Dryrun", func() {
		var log *logger.FunLogger

		BeforeEach(func() {
			log = logger.NewLogger()
		})

		Context("with Kubernetes validation", func() {
			It("should succeed with valid version format", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kubeadm",
							KubernetesVersion:   "v1.28.0",
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with empty version", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kubeadm",
							KubernetesVersion:   "",
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with invalid version format (no v prefix)", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: "kubeadm",
							KubernetesVersion:   "1.28.0",
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("vX.Y.Z"))
			})
		})

		Context("with Container Runtime validation", func() {
			It("should warn when no runtime specified", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    "",
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with containerd", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeContainerd,
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with crio", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeCrio,
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with docker", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    v1alpha1.ContainerRuntimeDocker,
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with unsupported runtime", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Install: true,
							Name:    "unsupported",
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not supported"))
			})
		})

		Context("with CTK validation", func() {
			It("should succeed when CTK not installed", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: false,
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed with valid CTK config", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourcePackage,
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with invalid CTK config", func() {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						NVIDIAContainerToolkit: v1alpha1.NVIDIAContainerToolkit{
							Install: true,
							Source:  v1alpha1.CTKSourceGit,
							// Missing git config
						},
					},
				}
				err := Dryrun(log, env)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with empty environment", func() {
			It("should succeed", func() {
				env := v1alpha1.Environment{}
				err := Dryrun(log, env)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
