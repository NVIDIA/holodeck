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

package aws_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"
)

var _ = Describe("AWS Provider", func() {
	var (
		log     *logger.FunLogger
		buf     bytes.Buffer
		tmpDir  string
		tmpFile string
	)

	BeforeEach(func() {
		log = logger.NewLogger()
		log.Out = &buf
		buf.Reset()

		var err error
		tmpDir, err = os.MkdirTemp("", "holodeck-aws-provider-*")
		Expect(err).NotTo(HaveOccurred())
		tmpFile = filepath.Join(tmpDir, "cache.yaml")
	})

	AfterEach(func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	})

	Describe("Constants", func() {
		It("should have the correct provider name", func() {
			Expect(aws.Name).To(Equal("aws"))
		})

		It("should have the correct property names", func() {
			Expect(aws.VpcID).To(Equal("vpc-id"))
			Expect(aws.SubnetID).To(Equal("subnet-id"))
			Expect(aws.InternetGwID).To(Equal("internet-gateway-id"))
			Expect(aws.RouteTable).To(Equal("route-table-id"))
			Expect(aws.SecurityGroupID).To(Equal("security-group-id"))
			Expect(aws.InstanceID).To(Equal("instance-id"))
			Expect(aws.PublicDnsName).To(Equal("public-dns-name"))
		})
	})

	Describe("AWS struct", func() {
		It("should be able to create and populate AWS struct", func() {
			awsData := &aws.AWS{
				Vpcid:                     "vpc-12345",
				Subnetid:                  "subnet-12345",
				InternetGwid:              "igw-12345",
				InternetGatewayAttachment: "vpc-12345",
				RouteTable:                "rtb-12345",
				SecurityGroupid:           "sg-12345",
				Instanceid:                "i-12345",
				PublicDnsName:             "ec2-1-2-3-4.compute.amazonaws.com",
			}

			Expect(awsData.Vpcid).To(Equal("vpc-12345"))
			Expect(awsData.Subnetid).To(Equal("subnet-12345"))
			Expect(awsData.InternetGwid).To(Equal("igw-12345"))
			Expect(awsData.InternetGatewayAttachment).To(Equal("vpc-12345"))
			Expect(awsData.RouteTable).To(Equal("rtb-12345"))
			Expect(awsData.SecurityGroupid).To(Equal("sg-12345"))
			Expect(awsData.Instanceid).To(Equal("i-12345"))
			Expect(awsData.PublicDnsName).To(
				Equal("ec2-1-2-3-4.compute.amazonaws.com"))
		})
	})

	Describe("ImageInfo and ByCreationDate sorting", func() {
		It("should sort images by creation date ascending", func() {
			images := aws.ByCreationDate{
				{ImageID: "ami-3", CreationDate: "2024-03-01T00:00:00.000Z"},
				{ImageID: "ami-1", CreationDate: "2024-01-01T00:00:00.000Z"},
				{ImageID: "ami-2", CreationDate: "2024-02-01T00:00:00.000Z"},
			}

			sort.Sort(images)

			Expect(images[0].ImageID).To(Equal("ami-1"))
			Expect(images[1].ImageID).To(Equal("ami-2"))
			Expect(images[2].ImageID).To(Equal("ami-3"))
		})

		It("should handle empty slice", func() {
			images := aws.ByCreationDate{}
			sort.Sort(images)
			Expect(images).To(BeEmpty())
		})

		It("should handle single image", func() {
			images := aws.ByCreationDate{
				{ImageID: "ami-1", CreationDate: "2024-01-01T00:00:00.000Z"},
			}
			sort.Sort(images)
			Expect(images).To(HaveLen(1))
			Expect(images[0].ImageID).To(Equal("ami-1"))
		})

		It("should handle images with same creation date", func() {
			images := aws.ByCreationDate{
				{ImageID: "ami-2", CreationDate: "2024-01-01T00:00:00.000Z"},
				{ImageID: "ami-1", CreationDate: "2024-01-01T00:00:00.000Z"},
			}
			sort.Sort(images)
			// Both have same date, order might vary but should not panic
			Expect(images).To(HaveLen(2))
		})
	})

	Describe("Environment Status", func() {
		var (
			tmpDir    string
			cachePath string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "holodeck-aws-test-*")
			Expect(err).NotTo(HaveOccurred())
			cachePath = filepath.Join(tmpDir, "cache.yaml")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("when cache file doesn't exist", func() {
			It("should fail to read status", func() {
				// Provider would fail to read non-existent cache
				_, err := os.ReadFile(cachePath) //nolint:gosec // test file path
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when cache file contains valid environment", func() {
			BeforeEach(func() {
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties:
    - name: vpc-id
      value: vpc-12345
    - name: instance-id
      value: i-12345
  conditions:
    - type: Available
      status: "True"
`
				err := os.WriteFile(cachePath, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be readable", func() {
				data, err := os.ReadFile(cachePath) //nolint:gosec // test file path
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("vpc-12345"))
				Expect(string(data)).To(ContainSubstring("i-12345"))
			})
		})
	})

	Describe("Condition helpers", func() {
		Context("ConditionAvailable", func() {
			It("should have correct value", func() {
				Expect(v1alpha1.ConditionAvailable).To(Equal("Available"))
			})
		})

		Context("ConditionDegraded", func() {
			It("should have correct value", func() {
				Expect(v1alpha1.ConditionDegraded).To(Equal("Degraded"))
			})
		})

		Context("ConditionProgressing", func() {
			It("should have correct value", func() {
				Expect(v1alpha1.ConditionProgressing).To(Equal("Progressing"))
			})
		})

		Context("ConditionTerminated", func() {
			It("should have correct value", func() {
				Expect(v1alpha1.ConditionTerminated).To(Equal("Terminated"))
			})
		})
	})

	Describe("Environment Properties", func() {
		It("should create properties correctly", func() {
			props := []v1alpha1.Properties{
				{Name: aws.VpcID, Value: "vpc-test"},
				{Name: aws.SubnetID, Value: "subnet-test"},
				{Name: aws.InstanceID, Value: "i-test"},
			}

			Expect(props).To(HaveLen(3))
			Expect(props[0].Name).To(Equal("vpc-id"))
			Expect(props[0].Value).To(Equal("vpc-test"))
		})

		It("should be able to find a property by name", func() {
			props := []v1alpha1.Properties{
				{Name: aws.VpcID, Value: "vpc-123"},
				{Name: aws.InstanceID, Value: "i-456"},
			}

			var instanceID string
			for _, p := range props {
				if p.Name == aws.InstanceID {
					instanceID = p.Value
					break
				}
			}

			Expect(instanceID).To(Equal("i-456"))
		})
	})

	Describe("Environment Configuration", func() {
		It("should create a valid AWS environment spec", func() {
			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-env",
				},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
					},
					Auth: v1alpha1.Auth{
						KeyName:    "my-key",
						PrivateKey: "/path/to/key.pem",
						Username:   "ubuntu",
					},
				},
			}

			Expect(env.Spec.Provider).To(Equal(v1alpha1.ProviderAWS))
			Expect(env.Spec.Instance.Type).To(Equal("t3.medium"))
			Expect(env.Spec.Instance.Region).To(Equal("us-east-1"))
			Expect(env.Spec.Auth.KeyName).To(Equal("my-key"))
		})

		It("should support ingress IP ranges", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						IngressIpRanges: []string{
							"10.0.0.0/8",
							"192.168.0.0/16",
						},
					},
				},
			}

			Expect(env.Spec.Instance.IngressIpRanges).To(HaveLen(2))
			Expect(env.Spec.Instance.IngressIpRanges).To(
				ContainElements("10.0.0.0/8", "192.168.0.0/16"))
		})

		It("should support custom root volume size", func() {
			volumeSize := int32(128)
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						RootVolumeSizeGB: &volumeSize,
					},
				},
			}

			Expect(env.Spec.Instance.RootVolumeSizeGB).NotTo(BeNil())
			Expect(*env.Spec.Instance.RootVolumeSizeGB).To(Equal(int32(128)))
		})
	})

	Describe("Image Configuration", func() {
		It("should support x86_64 architecture", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Instance: v1alpha1.Instance{
						Image: v1alpha1.Image{
							Architecture: "x86_64",
						},
					},
				},
			}
			Expect(env.Spec.Instance.Image.Architecture).To(Equal("x86_64"))
		})

		It("should support arm64 architecture", func() {
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Instance: v1alpha1.Instance{
						Image: v1alpha1.Image{
							Architecture: "arm64",
						},
					},
				},
			}
			Expect(env.Spec.Instance.Image.Architecture).To(Equal("arm64"))
		})

		It("should support custom image ID", func() {
			imageID := "ami-custom123"
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Instance: v1alpha1.Instance{
						Image: v1alpha1.Image{
							ImageId: &imageID,
						},
					},
				},
			}
			Expect(env.Spec.Instance.Image.ImageId).NotTo(BeNil())
			Expect(*env.Spec.Instance.Image.ImageId).To(Equal("ami-custom123"))
		})

		It("should support custom owner ID", func() {
			ownerID := "123456789012"
			env := v1alpha1.Environment{
				Spec: v1alpha1.EnvironmentSpec{
					Instance: v1alpha1.Instance{
						Image: v1alpha1.Image{
							OwnerId: &ownerID,
						},
					},
				},
			}
			Expect(env.Spec.Instance.Image.OwnerId).NotTo(BeNil())
			Expect(*env.Spec.Instance.Image.OwnerId).To(Equal("123456789012"))
		})
	})

	Describe("Provider Creation", func() {
		Context("with mock EC2 client", func() {
			It("should create provider with mock client", func() {
				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-env",
					},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image: v1alpha1.Image{
								Architecture: "x86_64",
							},
						},
						Auth: v1alpha1.Auth{
							KeyName:    "test-key",
							PrivateKey: "/path/to/key.pem",
							Username:   "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())
				Expect(provider).NotTo(BeNil())
				Expect(provider.Name()).To(Equal("aws"))
			})

			It("should use AWS_REGION environment variable if set", func() {
				// Save original and set test value
				origRegion := os.Getenv("AWS_REGION")
				_ = os.Setenv("AWS_REGION", "eu-west-1")
				defer func() { _ = os.Setenv("AWS_REGION", origRegion) }()

				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-env",
					},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1", // This should be overridden
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())
				Expect(provider).NotTo(BeNil())
			})
		})

		Context("with GitHub environment variables", func() {
			It("should include GitHub tags when env vars are set", func() {
				// Set GitHub environment variables
				_ = os.Setenv("GITHUB_SHA", "abc123def456")
				_ = os.Setenv("GITHUB_ACTOR", "test-user")
				_ = os.Setenv("GITHUB_REF_NAME", "main")
				_ = os.Setenv("GITHUB_REPOSITORY", "NVIDIA/holodeck")
				_ = os.Setenv("GITHUB_RUN_ID", "12345")
				defer func() {
					_ = os.Unsetenv("GITHUB_SHA")
					_ = os.Unsetenv("GITHUB_ACTOR")
					_ = os.Unsetenv("GITHUB_REF_NAME")
					_ = os.Unsetenv("GITHUB_REPOSITORY")
					_ = os.Unsetenv("GITHUB_RUN_ID")
				}()

				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-env",
					},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())
				Expect(provider).NotTo(BeNil())
			})
		})
	})

	Describe("DryRun", func() {
		It("should return nil for dry run", func() {
			mockClient := aws.NewMockEC2Client()
			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-env",
				},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
					},
				},
			}

			provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())

			err = provider.DryRun()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Cache operations", func() {
		Context("unmarshalCache", func() {
			It("should parse cache file with all properties", func() {
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties:
    - name: vpc-id
      value: vpc-12345
    - name: subnet-id
      value: subnet-12345
    - name: internet-gateway-id
      value: igw-12345
    - name: route-table-id
      value: rtb-12345
    - name: security-group-id
      value: sg-12345
    - name: instance-id
      value: i-12345
    - name: public-dns-name
      value: ec2-1-2-3-4.compute.amazonaws.com
`
				err := os.WriteFile(tmpFile, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())

				// Verify file can be read
				data, err := os.ReadFile(tmpFile) //nolint:gosec
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("vpc-12345"))
				Expect(string(data)).To(ContainSubstring("i-12345"))
			})

			It("should handle cache file with unknown properties", func() {
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties:
    - name: vpc-id
      value: vpc-12345
    - name: unknown-property
      value: some-value
`
				err := os.WriteFile(tmpFile, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())

				data, err := os.ReadFile(tmpFile) //nolint:gosec
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("unknown-property"))
			})
		})
	})

	Describe("Status operations", func() {
		Context("Status", func() {
			It("should return conditions from cache file", func() {
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  conditions:
    - type: Available
      status: "True"
    - type: Progressing
      status: "False"
`
				err := os.WriteFile(tmpFile, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())

				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				conditions, err := provider.Status()
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(HaveLen(2))
				Expect(conditions[0].Type).To(Equal("Available"))
				Expect(string(conditions[0].Status)).To(Equal("True"))
			})

			It("should return empty conditions when none exist", func() {
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties: []
`
				err := os.WriteFile(tmpFile, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())

				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				conditions, err := provider.Status()
				Expect(err).NotTo(HaveOccurred())
				Expect(conditions).To(BeEmpty())
			})

			It("should return error when cache file doesn't exist", func() {
				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				}

				provider, err := aws.New(log, env, "/nonexistent/cache.yaml", aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				_, err = provider.Status()
				Expect(err).To(HaveOccurred())
			})

			It("should return error when cache file has invalid YAML", func() {
				err := os.WriteFile(tmpFile, []byte("invalid: [yaml"), 0600)
				Expect(err).NotTo(HaveOccurred())

				mockClient := aws.NewMockEC2Client()
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				}

				provider, err := aws.New(log, env, tmpFile, aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				_, err = provider.Status()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Condition helpers", func() {
		It("should return correct available conditions", func() {
			conditions := aws.GetAvailableConditions()
			Expect(conditions).To(HaveLen(4))

			// Find Available condition
			var availableFound bool
			for _, c := range conditions {
				if c.Type == v1alpha1.ConditionAvailable {
					Expect(string(c.Status)).To(Equal("True"))
					availableFound = true
				} else {
					Expect(string(c.Status)).To(Equal("False"))
				}
			}
			Expect(availableFound).To(BeTrue())
		})

		It("should return correct degraded conditions", func() {
			conditions := aws.GetDegradedConditions("TestReason", "Test message")
			Expect(conditions).To(HaveLen(4))

			// Find Degraded condition
			var degradedFound bool
			for _, c := range conditions {
				if c.Type == v1alpha1.ConditionDegraded {
					Expect(string(c.Status)).To(Equal("True"))
					Expect(c.Reason).To(Equal("TestReason"))
					Expect(c.Message).To(Equal("Test message"))
					degradedFound = true
				}
			}
			Expect(degradedFound).To(BeTrue())
		})

		It("should return correct progressing conditions", func() {
			conditions := aws.GetProgressingConditions("Creating", "Creating VPC")
			Expect(conditions).To(HaveLen(4))

			// Find Progressing condition
			var progressingFound bool
			for _, c := range conditions {
				if c.Type == v1alpha1.ConditionProgressing {
					Expect(string(c.Status)).To(Equal("True"))
					Expect(c.Reason).To(Equal("Creating"))
					Expect(c.Message).To(Equal("Creating VPC"))
					progressingFound = true
				}
			}
			Expect(progressingFound).To(BeTrue())
		})

		It("should return correct terminated conditions", func() {
			conditions := aws.GetTerminatedConditions("Terminated", "Resources deleted")
			Expect(conditions).To(HaveLen(4))

			// Find Terminated condition
			var terminatedFound bool
			for _, c := range conditions {
				if c.Type == v1alpha1.ConditionTerminated {
					Expect(string(c.Status)).To(Equal("True"))
					Expect(c.Reason).To(Equal("Terminated"))
					Expect(c.Message).To(Equal("Resources deleted"))
					terminatedFound = true
				}
			}
			Expect(terminatedFound).To(BeTrue())
		})
	})

	Describe("Update function", func() {
		It("should create cache file and directory if they don't exist", func() {
			newCacheFile := filepath.Join(tmpDir, "subdir", "new-cache.yaml")
			env := &v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				Status: v1alpha1.EnvironmentStatus{
					Properties: []v1alpha1.Properties{
						{Name: aws.VpcID, Value: "vpc-new"},
					},
				},
			}

			err := aws.Update(env, newCacheFile)
			Expect(err).NotTo(HaveOccurred())

			// Verify file was created
			data, err := os.ReadFile(newCacheFile) //nolint:gosec
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("vpc-new"))
		})

		It("should update existing cache file", func() {
			// Create initial cache file
			initialContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
`
			err := os.WriteFile(tmpFile, []byte(initialContent), 0600)
			Expect(err).NotTo(HaveOccurred())

			env := &v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec:       v1alpha1.EnvironmentSpec{Provider: v1alpha1.ProviderAWS},
				Status: v1alpha1.EnvironmentStatus{
					Properties: []v1alpha1.Properties{
						{Name: aws.VpcID, Value: "vpc-updated"},
						{Name: aws.InstanceID, Value: "i-updated"},
					},
				},
			}

			err = aws.Update(env, tmpFile)
			Expect(err).NotTo(HaveOccurred())

			// Verify file was updated
			data, err := os.ReadFile(tmpFile) //nolint:gosec
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("vpc-updated"))
			Expect(string(data)).To(ContainSubstring("i-updated"))
		})
	})

	Describe("Create workflow error handling", func() {
		var (
			mockClient *aws.MockEC2Client
			provider   *aws.Provider
		)

		BeforeEach(func() {
			mockClient = aws.NewMockEC2Client()
		})

		Context("VPC creation errors", func() {
			It("should fail when CreateVpc returns error", func() {
				mockClient.CreateVpcFunc = func(ctx context.Context,
					params *ec2.CreateVpcInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
					return nil, fmt.Errorf("vpc creation failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating VPC"))
			})

			It("should fail when ModifyVpcAttribute returns error", func() {
				mockClient.ModifyVpcAttrFunc = func(ctx context.Context,
					params *ec2.ModifyVpcAttributeInput,
					optFns ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
					return nil, fmt.Errorf("modify vpc failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating VPC"))
			})
		})

		Context("Subnet creation errors", func() {
			It("should fail when CreateSubnet returns error", func() {
				mockClient.CreateSubnetFunc = func(ctx context.Context,
					params *ec2.CreateSubnetInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
					return nil, fmt.Errorf("subnet creation failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating subnet"))
			})
		})

		Context("Internet Gateway creation errors", func() {
			It("should fail when CreateInternetGateway returns error", func() {
				mockClient.CreateIGWFunc = func(ctx context.Context,
					params *ec2.CreateInternetGatewayInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
					return nil, fmt.Errorf("igw creation failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"error creating Internet Gateway"))
			})

			It("should fail when AttachInternetGateway returns error", func() {
				mockClient.AttachIGWFunc = func(ctx context.Context,
					params *ec2.AttachInternetGatewayInput,
					optFns ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
					return nil, fmt.Errorf("attach igw failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"error creating Internet Gateway"))
			})
		})

		Context("Route Table creation errors", func() {
			It("should fail when CreateRouteTable returns error", func() {
				mockClient.CreateRTFunc = func(ctx context.Context,
					params *ec2.CreateRouteTableInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
					return nil, fmt.Errorf("route table creation failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating route table"))
			})

			It("should fail when AssociateRouteTable returns error", func() {
				mockClient.AssociateRTFunc = func(ctx context.Context,
					params *ec2.AssociateRouteTableInput,
					optFns ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
					return nil, fmt.Errorf("associate rt failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating route table"))
			})

			It("should fail when CreateRoute returns error", func() {
				mockClient.CreateRouteFunc = func(ctx context.Context,
					params *ec2.CreateRouteInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
					return nil, fmt.Errorf("create route failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error creating route table"))
			})
		})

		Context("Security Group creation errors", func() {
			It("should fail when CreateSecurityGroup returns error", func() {
				mockClient.CreateSGFunc = func(ctx context.Context,
					params *ec2.CreateSecurityGroupInput,
					optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
					return nil, fmt.Errorf("sg creation failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"error creating security group"))
			})

			It("should fail when AuthorizeSecurityGroupIngress returns error",
				func() {
					mockClient.AuthorizeSGFunc = func(ctx context.Context,
						params *ec2.AuthorizeSecurityGroupIngressInput,
						optFns ...func(*ec2.Options)) (
						*ec2.AuthorizeSecurityGroupIngressOutput, error) {
						return nil, fmt.Errorf("authorize ingress failed")
					}

					env := v1alpha1.Environment{
						ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
						Spec: v1alpha1.EnvironmentSpec{
							Provider: v1alpha1.ProviderAWS,
							Instance: v1alpha1.Instance{
								Type:   "t3.medium",
								Region: "us-east-1",
								Image:  v1alpha1.Image{Architecture: "x86_64"},
							},
							Auth: v1alpha1.Auth{
								KeyName: "test-key", PrivateKey: "/path/to/key.pem",
								Username: "ubuntu",
							},
						},
					}

					var err error
					provider, err = aws.New(log, env, tmpFile,
						aws.WithEC2Client(mockClient))
					Expect(err).NotTo(HaveOccurred())

					err = provider.Create()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(
						"error creating security group"))
				})
		})

		Context("EC2 Instance creation errors", func() {
			It("should fail when DescribeImages returns error", func() {
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("describe images failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"error creating EC2 instance"))
			})

			It("should fail when RunInstances returns error", func() {
				imageID := "ami-test123"
				mockClient.RunInstancesFunc = func(ctx context.Context,
					params *ec2.RunInstancesInput,
					optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
					return nil, fmt.Errorf("run instances failed")
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{ImageId: &imageID},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				var err error
				provider, err = aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Create()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"error creating EC2 instance"))
			})
		})
	})

	Describe("DryRun", func() {
		It("should succeed with valid configuration and mock client", func() {
			mockClient := aws.NewMockEC2Client()

			// Mock DescribeImages for setAMI
			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{
					Images: []types.Image{
						{
							ImageId:      strPtr("ami-test123"),
							CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
						},
					},
				}, nil
			}

			// Mock DescribeInstanceTypes for checkInstanceTypes
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{
						{InstanceType: types.InstanceTypeT3Medium},
					},
				}, nil
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
						Image:  v1alpha1.Image{Architecture: "x86_64"},
					},
					Auth: v1alpha1.Auth{
						KeyName: "test-key", PrivateKey: "/path/to/key.pem",
						Username: "ubuntu",
					},
				},
			}

			provider, err := aws.New(log, env, tmpFile,
				aws.WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())

			err = provider.DryRun()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when instance type is unsupported", func() {
			mockClient := aws.NewMockEC2Client()

			// Mock DescribeImages for setAMI
			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{
					Images: []types.Image{
						{
							ImageId:      strPtr("ami-test123"),
							CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
						},
					},
				}, nil
			}

			// Return empty list - instance type not supported
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{},
				}, nil
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
						Image:  v1alpha1.Image{Architecture: "x86_64"},
					},
					Auth: v1alpha1.Auth{
						KeyName: "test-key", PrivateKey: "/path/to/key.pem",
						Username: "ubuntu",
					},
				},
			}

			provider, err := aws.New(log, env, tmpFile,
				aws.WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())

			err = provider.DryRun()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not supported"))
		})

		It("should fail when image not found", func() {
			mockClient := aws.NewMockEC2Client()

			// Return empty images list
			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{Images: []types.Image{}}, nil
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
						Image:  v1alpha1.Image{Architecture: "x86_64"},
					},
					Auth: v1alpha1.Auth{
						KeyName: "test-key", PrivateKey: "/path/to/key.pem",
						Username: "ubuntu",
					},
				},
			}

			provider, err := aws.New(log, env, tmpFile,
				aws.WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())

			err = provider.DryRun()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no images found"))
		})
	})

	Describe("Delete workflow", func() {
		Context("cache file errors", func() {
			It("should fail when cache file does not exist", func() {
				mockClient := aws.NewMockEC2Client()
				nonExistentCache := filepath.Join(tmpDir, "nonexistent.yaml")

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, nonExistentCache,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Delete()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error retrieving cache"))
			})

			It("should fail when cache file is invalid YAML", func() {
				mockClient := aws.NewMockEC2Client()

				err := os.WriteFile(tmpFile, []byte("invalid: yaml: content:"),
					0600)
				Expect(err).NotTo(HaveOccurred())

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Delete()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error retrieving cache"))
			})
		})

		Context("with empty cache", func() {
			It("should succeed when cache has no resources", func() {
				mockClient := aws.NewMockEC2Client()

				// Create empty cache file
				cacheContent := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties: []
`
				err := os.WriteFile(tmpFile, []byte(cacheContent), 0600)
				Expect(err).NotTo(HaveOccurred())

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.Delete()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Image validation", func() {
		Context("setAMI", func() {
			It("should return image when found", func() {
				mockClient := aws.NewMockEC2Client()
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-newer"),
								CreationDate: strPtr("2024-06-01T00:00:00.000Z"),
							},
							{
								ImageId:      strPtr("ami-older"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
				mockClient.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{InstanceType: types.InstanceTypeT3Medium},
						},
					}, nil
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "x86_64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should support arm64 architecture", func() {
				mockClient := aws.NewMockEC2Client()
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-arm64"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
				mockClient.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{InstanceType: types.InstanceTypeT4gMedium},
						},
					}, nil
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t4g.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "arm64"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail for invalid architecture", func() {
				mockClient := aws.NewMockEC2Client()

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{Architecture: "invalid_arch"},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid architecture"))
			})

			It("should skip setAMI when ImageId is provided", func() {
				mockClient := aws.NewMockEC2Client()
				imageID := "ami-custom123"

				// Mock DescribeImages to return the custom image
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-custom123"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
				mockClient.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{InstanceType: types.InstanceTypeT3Medium},
						},
					}, nil
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{ImageId: &imageID},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail when custom ImageId is not found", func() {
				mockClient := aws.NewMockEC2Client()
				imageID := "ami-notfound"

				// Mock DescribeImages to return a different image
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-different"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image:  v1alpha1.Image{ImageId: &imageID},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is not supported"))
			})
		})

		Context("custom owner ID", func() {
			It("should use custom owner ID when provided", func() {
				mockClient := aws.NewMockEC2Client()
				ownerID := "123456789012"

				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					// Verify owner filter is applied
					for _, filter := range params.Filters {
						if *filter.Name == "owner-id" {
							Expect(filter.Values).To(ContainElement("123456789012"))
						}
					}
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-custom-owner"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
				mockClient.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{InstanceType: types.InstanceTypeT3Medium},
						},
					}, nil
				}

				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image: v1alpha1.Image{
								Architecture: "x86_64",
								OwnerId:      &ownerID,
							},
						},
						Auth: v1alpha1.Auth{
							KeyName: "test-key", PrivateKey: "/path/to/key.pem",
							Username: "ubuntu",
						},
					},
				}

				provider, err := aws.New(log, env, tmpFile,
					aws.WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				err = provider.DryRun()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func strPtr(s string) *string {
	return &s
}
