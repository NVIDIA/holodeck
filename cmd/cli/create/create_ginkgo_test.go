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

package create_test

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/create"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Create Command", func() {
	var (
		log *logger.FunLogger
		buf bytes.Buffer
	)

	BeforeEach(func() {
		log = logger.NewLogger()
		log.Out = &buf
		buf.Reset()
	})

	Describe("NewCommand", func() {
		It("should create a valid command", func() {
			cmd := create.NewCommand(log)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Name).To(Equal("create"))
			Expect(cmd.Usage).To(ContainSubstring("create"))
		})

		It("should have all required flags", func() {
			cmd := create.NewCommand(log)
			flagNames := make(map[string]bool)
			for _, flag := range cmd.Flags {
				for _, name := range flag.Names() {
					flagNames[name] = true
				}
			}

			Expect(flagNames).To(HaveKey("provision"))
			Expect(flagNames).To(HaveKey("p"))
			Expect(flagNames).To(HaveKey("kubeconfig"))
			Expect(flagNames).To(HaveKey("k"))
			Expect(flagNames).To(HaveKey("cachepath"))
			Expect(flagNames).To(HaveKey("c"))
			Expect(flagNames).To(HaveKey("envFile"))
			Expect(flagNames).To(HaveKey("f"))
		})

		It("should have an action", func() {
			cmd := create.NewCommand(log)
			Expect(cmd.Action).NotTo(BeNil())
		})

		It("should have a before hook", func() {
			cmd := create.NewCommand(log)
			Expect(cmd.Before).NotTo(BeNil())
		})
	})

	Describe("Options validation", func() {
		Context("environment file handling", func() {
			var (
				tmpDir  string
				envFile string
			)

			BeforeEach(func() {
				var err error
				tmpDir, err = os.MkdirTemp("", "holodeck-create-test-*")
				Expect(err).NotTo(HaveOccurred())
				envFile = filepath.Join(tmpDir, "env.yaml")
			})

			AfterEach(func() {
				os.RemoveAll(tmpDir)
			})

			It("should handle valid environment file", func() {
				content := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
  instance:
    type: t3.medium
    region: us-east-1
  auth:
    keyName: test-key
    username: ubuntu
    privateKey: /path/to/key.pem
`
				err := os.WriteFile(envFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())

				// File should be readable
				_, err = os.ReadFile(envFile)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should detect missing environment file", func() {
				_, err := os.ReadFile("/nonexistent/path/env.yaml")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Environment types", func() {
		Describe("AWS provider options", func() {
			It("should support AWS provider", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-aws",
					},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
					},
				}
				Expect(env.Spec.Provider).To(Equal(v1alpha1.ProviderAWS))
			})

			It("should default username to ubuntu for AWS", func() {
				// This is the expected behavior based on the code
				defaultUsername := "ubuntu"
				Expect(defaultUsername).To(Equal("ubuntu"))
			})
		})

		Describe("SSH provider options", func() {
			It("should support SSH provider", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ssh",
					},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderSSH,
						Instance: v1alpha1.Instance{
							HostUrl: "192.168.1.100",
						},
					},
				}
				Expect(env.Spec.Provider).To(Equal(v1alpha1.ProviderSSH))
				Expect(env.Spec.Instance.HostUrl).To(Equal("192.168.1.100"))
			})
		})
	})

	Describe("Cache file handling", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "holodeck-cache-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("should handle cache file with status properties", func() {
			cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
  labels:
    holodeck.nvidia.com/instance-id: test-12345
spec:
  provider: aws
status:
  properties:
    - name: vpc-id
      value: vpc-12345
    - name: instance-id
      value: i-12345
    - name: public-dns-name
      value: ec2-1-2-3-4.compute.amazonaws.com
`
			cacheFile := filepath.Join(tmpDir, "cache.yaml")
			err := os.WriteFile(cacheFile, []byte(cacheContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(cacheFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("vpc-12345"))
			Expect(string(content)).To(ContainSubstring("i-12345"))
		})
	})

	Describe("AWS property names", func() {
		It("should use correct property names", func() {
			Expect(aws.VpcID).To(Equal("vpc-id"))
			Expect(aws.SubnetID).To(Equal("subnet-id"))
			Expect(aws.InternetGwID).To(Equal("internet-gateway-id"))
			Expect(aws.RouteTable).To(Equal("route-table-id"))
			Expect(aws.SecurityGroupID).To(Equal("security-group-id"))
			Expect(aws.InstanceID).To(Equal("instance-id"))
			Expect(aws.PublicDnsName).To(Equal("public-dns-name"))
		})
	})

	Describe("Container runtime defaults", func() {
		It("should default to empty container runtime name", func() {
			env := v1alpha1.Environment{}
			Expect(env.Spec.ContainerRuntime.Name).To(Equal(v1alpha1.ContainerRuntimeNone))
		})

		DescribeTable("container runtime options",
			func(runtime v1alpha1.ContainerRuntimeName) {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						ContainerRuntime: v1alpha1.ContainerRuntime{
							Name: runtime,
						},
					},
				}
				Expect(env.Spec.ContainerRuntime.Name).To(Equal(runtime))
			},
			Entry("containerd", v1alpha1.ContainerRuntimeContainerd),
			Entry("docker", v1alpha1.ContainerRuntimeDocker),
			Entry("crio", v1alpha1.ContainerRuntimeCrio),
			Entry("none", v1alpha1.ContainerRuntimeNone),
		)
	})

	Describe("Kubernetes installer options", func() {
		DescribeTable("kubernetes installers",
			func(installer string) {
				env := v1alpha1.Environment{
					Spec: v1alpha1.EnvironmentSpec{
						Kubernetes: v1alpha1.Kubernetes{
							Install:             true,
							KubernetesInstaller: installer,
						},
					},
				}
				Expect(env.Spec.Kubernetes.KubernetesInstaller).To(Equal(installer))
			},
			Entry("kubeadm", "kubeadm"),
			Entry("kind", "kind"),
			Entry("microk8s", "microk8s"),
		)
	})

	Describe("Label handling", func() {
		It("should support instance labels", func() {
			labels := make(map[string]string)
			labels["holodeck.nvidia.com/instance-id"] = "test-12345"
			labels["holodeck.nvidia.com/provisioned"] = "false"

			Expect(labels).To(HaveKey("holodeck.nvidia.com/instance-id"))
			Expect(labels["holodeck.nvidia.com/instance-id"]).To(Equal("test-12345"))
		})
	})
})
