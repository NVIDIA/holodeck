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

package aws

import (
	"context"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
)

var _ = Describe("AWS Provider", func() {
	var (
		mockClient *MockEC2Client
		log        *logger.FunLogger
	)

	BeforeEach(func() {
		mockClient = NewMockEC2Client()
		log = logger.NewLogger()
	})

	Describe("Name", func() {
		It("should return 'aws'", func() {
			Expect(Name).To(Equal("aws"))
		})
	})

	Describe("WithEC2Client option", func() {
		It("should inject the mock client", func() {
			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Region: "us-east-1",
					},
				},
			}

			tmpDir, err := os.MkdirTemp("", "aws-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(tmpDir) }()

			cacheFile := filepath.Join(tmpDir, "cache.yaml")

			provider, err := New(log, env, cacheFile, WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())
			Expect(provider).NotTo(BeNil())
			Expect(provider.ec2).To(Equal(mockClient))
		})
	})

	Describe("Constants", func() {
		It("should have correct property name constants", func() {
			Expect(VpcID).To(Equal("vpc-id"))
			Expect(SubnetID).To(Equal("subnet-id"))
			Expect(InternetGwID).To(Equal("internet-gateway-id"))
			Expect(InternetGatewayAttachment).To(Equal("internet-gateway-attachment-vpc-id"))
			Expect(RouteTable).To(Equal("route-table-id"))
			Expect(SecurityGroupID).To(Equal("security-group-id"))
			Expect(InstanceID).To(Equal("instance-id"))
			Expect(PublicDnsName).To(Equal("public-dns-name"))
		})
	})

	Describe("Provider.Name", func() {
		It("should return the provider name", func() {
			p := &Provider{}
			Expect(p.Name()).To(Equal("aws"))
		})
	})

	Describe("unmarsalCache", func() {
		var (
			tmpDir    string
			cacheFile string
			log       *logger.FunLogger
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")
			log = logger.NewLogger()
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("with valid cache file", func() {
			BeforeEach(func() {
				content := `apiVersion: holodeck.nvidia.com/v1alpha1
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
      value: subnet-67890
    - name: internet-gateway-id
      value: igw-11111
    - name: internet-gateway-attachment-vpc-id
      value: vpc-22222
    - name: route-table-id
      value: rtb-33333
    - name: security-group-id
      value: sg-44444
    - name: instance-id
      value: i-55555
    - name: public-dns-name
      value: ec2-1-2-3-4.compute.amazonaws.com
`
				err := os.WriteFile(cacheFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse all AWS properties", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				p := &Provider{
					cacheFile:   cacheFile,
					log:         log,
					Environment: &env,
				}

				aws, err := p.unmarsalCache()
				Expect(err).NotTo(HaveOccurred())
				Expect(aws.Vpcid).To(Equal("vpc-12345"))
				Expect(aws.Subnetid).To(Equal("subnet-67890"))
				Expect(aws.InternetGwid).To(Equal("igw-11111"))
				Expect(aws.InternetGatewayAttachment).To(Equal("vpc-22222"))
				Expect(aws.RouteTable).To(Equal("rtb-33333"))
				Expect(aws.SecurityGroupid).To(Equal("sg-44444"))
				Expect(aws.Instanceid).To(Equal("i-55555"))
				Expect(aws.PublicDnsName).To(Equal("ec2-1-2-3-4.compute.amazonaws.com"))
			})
		})

		Context("with cache file containing unknown properties", func() {
			BeforeEach(func() {
				content := `apiVersion: holodeck.nvidia.com/v1alpha1
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
      value: should-be-ignored
    - name: another-unknown
      value: also-ignored
`
				err := os.WriteFile(cacheFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should ignore unknown properties", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				p := &Provider{
					cacheFile:   cacheFile,
					log:         log,
					Environment: &env,
				}

				aws, err := p.unmarsalCache()
				Expect(err).NotTo(HaveOccurred())
				Expect(aws.Vpcid).To(Equal("vpc-12345"))
				// Other fields should be empty
				Expect(aws.Subnetid).To(BeEmpty())
				Expect(aws.Instanceid).To(BeEmpty())
			})
		})

		Context("with empty cache file", func() {
			BeforeEach(func() {
				content := `apiVersion: holodeck.nvidia.com/v1alpha1
kind: Environment
metadata:
  name: test-env
spec:
  provider: aws
status:
  properties: []
`
				err := os.WriteFile(cacheFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty AWS struct", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				p := &Provider{
					cacheFile:   cacheFile,
					log:         log,
					Environment: &env,
				}

				aws, err := p.unmarsalCache()
				Expect(err).NotTo(HaveOccurred())
				Expect(aws.Vpcid).To(BeEmpty())
				Expect(aws.Subnetid).To(BeEmpty())
			})
		})

		Context("with non-existent cache file", func() {
			It("should return an error", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				p := &Provider{
					cacheFile:   "/nonexistent/path/cache.yaml",
					log:         log,
					Environment: &env,
				}

				_, err := p.unmarsalCache()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with invalid YAML", func() {
			BeforeEach(func() {
				content := `not: valid: yaml: content`
				err := os.WriteFile(cacheFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				p := &Provider{
					cacheFile:   cacheFile,
					log:         log,
					Environment: &env,
				}

				_, err := p.unmarsalCache()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("AWS struct", func() {
		It("should have all required fields", func() {
			aws := AWS{
				Vpcid:                     "vpc-123",
				Subnetid:                  "subnet-456",
				InternetGwid:              "igw-789",
				InternetGatewayAttachment: "vpc-attach",
				RouteTable:                "rtb-111",
				SecurityGroupid:           "sg-222",
				Instanceid:                "i-333",
				PublicDnsName:             "ec2.example.com",
			}

			Expect(aws.Vpcid).To(Equal("vpc-123"))
			Expect(aws.Subnetid).To(Equal("subnet-456"))
			Expect(aws.InternetGwid).To(Equal("igw-789"))
			Expect(aws.InternetGatewayAttachment).To(Equal("vpc-attach"))
			Expect(aws.RouteTable).To(Equal("rtb-111"))
			Expect(aws.SecurityGroupid).To(Equal("sg-222"))
			Expect(aws.Instanceid).To(Equal("i-333"))
			Expect(aws.PublicDnsName).To(Equal("ec2.example.com"))
		})
	})

	Describe("checkInstanceTypes", func() {
		var (
			tmpDir    string
			cacheFile string
			provider  *Provider
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-instance-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
					},
				},
			}

			provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("should succeed when instance type is available", func() {
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{
						{InstanceType: types.InstanceTypeT3Medium,
							ProcessorInfo: &types.ProcessorInfo{
								SupportedArchitectures: []types.ArchitectureType{
									types.ArchitectureTypeX8664,
								},
							}},
					},
				}, nil
			}

			err := provider.checkInstanceTypes()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when instance type is not available", func() {
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{
						{InstanceType: types.InstanceTypeT3Large,
							ProcessorInfo: &types.ProcessorInfo{
								SupportedArchitectures: []types.ArchitectureType{
									types.ArchitectureTypeX8664,
								},
							}},
						{InstanceType: types.InstanceTypeT3Xlarge,
							ProcessorInfo: &types.ProcessorInfo{
								SupportedArchitectures: []types.ArchitectureType{
									types.ArchitectureTypeX8664,
								},
							}},
					},
				}, nil
			}

			err := provider.checkInstanceTypes()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not supported"))
		})
	})

	Describe("checkImages", func() {
		var (
			tmpDir    string
			cacheFile string
			provider  *Provider
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-image-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("when image ID is not specified", func() {
			BeforeEach(func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
						// No Image specified
					},
				}

				var err error
				provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find images by pattern", func() {
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-12345"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
								Name:         strPtr("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}

				err := provider.checkImages()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when image ID is specified", func() {
			BeforeEach(func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image: v1alpha1.Image{
								ImageId: strPtr("ami-specific-12345"),
							},
						},
					},
				}

				var err error
				provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should verify the specified image exists", func() {
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-specific-12345"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}

				err := provider.checkImages()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail when specified image does not exist", func() {
				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{},
					}, nil
				}

				err := provider.checkImages()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("DryRun", func() {
		var (
			tmpDir    string
			cacheFile string
			provider  *Provider
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-dryrun-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
					},
				},
			}

			provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("should succeed when instance type and images are valid", func() {
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{
						{InstanceType: types.InstanceTypeT3Medium,
							ProcessorInfo: &types.ProcessorInfo{
								SupportedArchitectures: []types.ArchitectureType{
									types.ArchitectureTypeX8664,
								},
							}},
					},
				}, nil
			}

			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{
					Images: []types.Image{
						{
							ImageId:      strPtr("ami-12345"),
							CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
							Name:         strPtr("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04"),
							Architecture: types.ArchitectureValuesX8664,
						},
					},
				}, nil
			}

			err := provider.DryRun()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when instance type check fails (triggers fail())", func() {
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				// Return empty list - instance type not found
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{},
				}, nil
			}

			err := provider.DryRun()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not supported"))
		})

		It("should fail when image check fails (triggers fail())", func() {
			mockClient.DescribeInstTypesFunc = func(ctx context.Context,
				params *ec2.DescribeInstanceTypesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
				return &ec2.DescribeInstanceTypesOutput{
					InstanceTypes: []types.InstanceTypeInfo{
						{InstanceType: types.InstanceTypeT3Medium,
							ProcessorInfo: &types.ProcessorInfo{
								SupportedArchitectures: []types.ArchitectureType{
									types.ArchitectureTypeX8664,
								},
							}},
					},
				}, nil
			}

			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				// Return error
				return nil, ErrMockDescribeImages
			}

			err := provider.DryRun()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get images"))
		})
	})

	Describe("setAMI", func() {
		var (
			tmpDir    string
			cacheFile string
			provider  *Provider
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-ami-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("when image ID is specified", func() {
			It("should use the specified AMI", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
							Image: v1alpha1.Image{
								ImageId: strPtr("ami-custom-12345"),
							},
						},
					},
				}

				var err error
				provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-custom-12345"),
								CreationDate: strPtr("2024-01-01T00:00:00.000Z"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}

				err = provider.setAMI()
				Expect(err).NotTo(HaveOccurred())
				Expect(*provider.Environment.Spec.Instance.Image.ImageId).To(
					Equal("ami-custom-12345"))
			})
		})

		Context("when image ID is not specified", func() {
			It("should find latest image by pattern", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
					},
				}

				var err error
				provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      strPtr("ami-older"),
								CreationDate: strPtr("2023-01-01T00:00:00.000Z"),
								Architecture: types.ArchitectureValuesX8664,
							},
							{
								ImageId:      strPtr("ami-latest"),
								CreationDate: strPtr("2024-06-01T00:00:00.000Z"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}

				err = provider.setAMI()
				Expect(err).NotTo(HaveOccurred())
				// Should select the latest image
				Expect(*provider.Environment.Spec.Instance.Image.ImageId).To(
					Equal("ami-latest"))
			})

			It("should fail when no images found", func() {
				env := v1alpha1.Environment{
					ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
					Spec: v1alpha1.EnvironmentSpec{
						Provider: v1alpha1.ProviderAWS,
						Instance: v1alpha1.Instance{
							Type:   "t3.medium",
							Region: "us-east-1",
						},
					},
				}

				var err error
				provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
				Expect(err).NotTo(HaveOccurred())

				mockClient.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{},
					}, nil
				}

				err = provider.setAMI()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no images found"))
			})
		})
	})

	Describe("describeImages", func() {
		var (
			tmpDir    string
			cacheFile string
			provider  *Provider
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "aws-describe-test-*")
			Expect(err).NotTo(HaveOccurred())
			cacheFile = filepath.Join(tmpDir, "cache.yaml")

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
					},
				},
			}

			provider, err = New(log, env, cacheFile, WithEC2Client(mockClient))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("should return error when DescribeImages fails", func() {
			mockClient.DescribeImagesFunc = func(ctx context.Context,
				params *ec2.DescribeImagesInput,
				optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
				return nil, ErrMockDescribeImages
			}

			filter := []types.Filter{
				{
					Name:   strPtr("image-id"),
					Values: []string{"ami-test"},
				},
			}
			_, err := provider.describeImages(filter)
			Expect(err).To(HaveOccurred())
		})
	})
})
