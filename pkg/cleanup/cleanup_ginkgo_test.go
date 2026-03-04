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

package cleanup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/testutil/mocks"
)

var _ = Describe("Cleanup Package", func() {

	Describe("safeString", func() {
		It("should return the string value when pointer is not nil", func() {
			s := "test-value"
			result := safeString(&s)
			Expect(result).To(Equal("test-value"))
		})

		It("should return '<nil>' when pointer is nil", func() {
			result := safeString(nil)
			Expect(result).To(Equal("<nil>"))
		})

		It("should handle empty string", func() {
			s := ""
			result := safeString(&s)
			Expect(result).To(Equal(""))
		})
	})

	Describe("Validation Patterns", func() {
		Describe("repoPattern", func() {
			DescribeTable("validating repository names",
				func(repo string, expected bool) {
					result := repoPattern.MatchString(repo)
					Expect(result).To(Equal(expected))
				},
				Entry("valid org/repo", "NVIDIA/holodeck", true),
				Entry("valid with hyphen", "my-org/my-repo", true),
				Entry("valid with underscore", "my_org/my_repo", true),
				Entry("valid with dot", "my.org/my.repo", true),
				Entry("valid with numbers", "org123/repo456", true),
				Entry("invalid - no slash", "holodeck", false),
				Entry("invalid - empty org", "/holodeck", false),
				Entry("invalid - empty repo", "NVIDIA/", false),
				Entry("invalid - spaces", "my org/my repo", false),
				Entry("invalid - special chars", "org@/repo!", false),
				Entry("invalid - multiple slashes", "org/sub/repo", false),
			)
		})

		Describe("runIDPattern", func() {
			DescribeTable("validating run IDs",
				func(runID string, expected bool) {
					result := runIDPattern.MatchString(runID)
					Expect(result).To(Equal(expected))
				},
				Entry("valid numeric ID", "12345678", true),
				Entry("valid single digit", "1", true),
				Entry("valid long ID", "1234567890123456789", true),
				Entry("invalid - contains letters", "123abc", false),
				Entry("invalid - empty", "", false),
				Entry("invalid - spaces", "123 456", false),
				Entry("invalid - special chars", "123-456", false),
			)
		})
	})

	Describe("GitHubJobsResponse", func() {
		It("should unmarshal valid JSON with completed jobs", func() {
			jsonData := `{
				"jobs": [
					{"status": "completed"},
					{"status": "completed"}
				]
			}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(HaveLen(2))
			Expect(resp.Jobs[0].Status).To(Equal("completed"))
			Expect(resp.Jobs[1].Status).To(Equal("completed"))
		})

		It("should unmarshal valid JSON with in_progress jobs", func() {
			jsonData := `{
				"jobs": [
					{"status": "in_progress"},
					{"status": "completed"}
				]
			}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(HaveLen(2))
			Expect(resp.Jobs[0].Status).To(Equal("in_progress"))
		})

		It("should handle empty jobs array", func() {
			jsonData := `{"jobs": []}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(BeEmpty())
		})
	})

	Describe("CheckGitHubJobsCompleted", func() {
		var (
			log     *logger.FunLogger
			buf     bytes.Buffer
			mockEC2 *mocks.MockEC2Client
		)

		BeforeEach(func() {
			log = logger.NewLogger()
			log.Out = &buf
			mockEC2 = &mocks.MockEC2Client{}
		})

		Context("input validation", func() {
			It("should reject invalid repository format", func() {
				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC2))
				Expect(err).NotTo(HaveOccurred())

				_, err = cleaner.CheckGitHubJobsCompleted(context.Background(), "invalid", "12345", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid repository format"))
			})

			It("should reject invalid runID format", func() {
				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC2))
				Expect(err).NotTo(HaveOccurred())

				_, err = cleaner.CheckGitHubJobsCompleted(context.Background(), "org/repo", "abc", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid runID format"))
			})

			It("should accept valid repository and runID formats", func() {
				// Test that valid formats pass validation (the HTTP request
				// will fail, but validation should pass)
				Expect(repoPattern.MatchString("NVIDIA/holodeck")).To(BeTrue())
				Expect(runIDPattern.MatchString("12345678")).To(BeTrue())
			})
		})
	})

	Describe("Job completion logic", func() {
		It("should consider all jobs completed when jobs array is empty", func() {
			resp := GitHubJobsResponse{Jobs: []struct {
				Status string `json:"status"`
			}{}}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeTrue())
		})

		It("should detect incomplete jobs", func() {
			resp := GitHubJobsResponse{
				Jobs: []struct {
					Status string `json:"status"`
				}{
					{Status: "completed"},
					{Status: "in_progress"},
					{Status: "completed"},
				},
			}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeFalse())
		})

		It("should confirm all jobs completed", func() {
			resp := GitHubJobsResponse{
				Jobs: []struct {
					Status string `json:"status"`
				}{
					{Status: "completed"},
					{Status: "completed"},
					{Status: "completed"},
				},
			}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeTrue())
		})
	})

	Describe("Cleaner with mock EC2", func() {
		var (
			log    *logger.FunLogger
			buf    bytes.Buffer
			mockEC *mocks.MockEC2Client
		)

		BeforeEach(func() {
			log = logger.NewLogger()
			log.Out = &buf
			mockEC = &mocks.MockEC2Client{}
		})

		Describe("New with WithEC2Client", func() {
			It("should create cleaner with injected EC2 client", func() {
				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())
				Expect(cleaner).NotTo(BeNil())
			})
		})

		Describe("GetTagValue", func() {
			It("should return tag value when found", func() {
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					return &ec2.DescribeTagsOutput{
						Tags: []types.TagDescription{
							{
								Key:   aws.String("GitHubRepository"),
								Value: aws.String("NVIDIA/holodeck"),
							},
						},
					}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				value, err := cleaner.GetTagValue(context.Background(), "vpc-12345", "GitHubRepository")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal("NVIDIA/holodeck"))
			})

			It("should return empty string when tag not found", func() {
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					return &ec2.DescribeTagsOutput{
						Tags: []types.TagDescription{},
					}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				value, err := cleaner.GetTagValue(context.Background(), "vpc-12345", "NonExistent")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(BeEmpty())
			})

			It("should return error when DescribeTags fails", func() {
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					return nil, errors.New("AWS error")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				_, err = cleaner.GetTagValue(context.Background(), "vpc-12345", "SomeKey")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to describe tags"))
			})
		})

		Describe("DeleteVPCResources", func() {
			BeforeEach(func() {
				// Set up default mock implementations for all required methods
				mockEC.DescribeInstancesFunc = func(ctx context.Context,
					params *ec2.DescribeInstancesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					return &ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{},
					}, nil
				}
				mockEC.DescribeSecurityGroupsFunc = func(ctx context.Context,
					params *ec2.DescribeSecurityGroupsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{
						SecurityGroups: []types.SecurityGroup{},
					}, nil
				}
				mockEC.DescribeSubnetsFunc = func(ctx context.Context,
					params *ec2.DescribeSubnetsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{
						Subnets: []types.Subnet{},
					}, nil
				}
				mockEC.DescribeRouteTablesFunc = func(ctx context.Context,
					params *ec2.DescribeRouteTablesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
					return &ec2.DescribeRouteTablesOutput{
						RouteTables: []types.RouteTable{},
					}, nil
				}
				mockEC.DescribeInternetGatewaysFunc = func(ctx context.Context,
					params *ec2.DescribeInternetGatewaysInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
					return &ec2.DescribeInternetGatewaysOutput{
						InternetGateways: []types.InternetGateway{},
					}, nil
				}
				mockEC.DeleteVpcFunc = func(ctx context.Context,
					params *ec2.DeleteVpcInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
					return &ec2.DeleteVpcOutput{}, nil
				}
			})

			It("should successfully delete empty VPC", func() {
				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail when DescribeInstances fails", func() {
				mockEC.DescribeInstancesFunc = func(ctx context.Context,
					params *ec2.DescribeInstancesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					return nil, errors.New("instance lookup failed")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete instances"))
			})

			It("should fail when DescribeSecurityGroups fails", func() {
				mockEC.DescribeSecurityGroupsFunc = func(ctx context.Context,
					params *ec2.DescribeSecurityGroupsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return nil, errors.New("security group lookup failed")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete security groups"))
			})

			It("should fail when DescribeSubnets fails", func() {
				mockEC.DescribeSubnetsFunc = func(ctx context.Context,
					params *ec2.DescribeSubnetsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return nil, errors.New("subnet lookup failed")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete subnets"))
			})

			It("should fail when DescribeRouteTables fails", func() {
				mockEC.DescribeRouteTablesFunc = func(ctx context.Context,
					params *ec2.DescribeRouteTablesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
					return nil, errors.New("route table lookup failed")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete route tables"))
			})

			It("should fail when DescribeInternetGateways fails", func() {
				mockEC.DescribeInternetGatewaysFunc = func(ctx context.Context,
					params *ec2.DescribeInternetGatewaysInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
					return nil, errors.New("gateway lookup failed")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete internet gateways"))
			})

			It("should delete subnets when present", func() {
				deletedSubnets := []string{}
				mockEC.DescribeSubnetsFunc = func(ctx context.Context,
					params *ec2.DescribeSubnetsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{
						Subnets: []types.Subnet{
							{SubnetId: aws.String("subnet-1")},
							{SubnetId: aws.String("subnet-2")},
						},
					}, nil
				}
				mockEC.DeleteSubnetFunc = func(ctx context.Context,
					params *ec2.DeleteSubnetInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
					deletedSubnets = append(deletedSubnets, *params.SubnetId)
					return &ec2.DeleteSubnetOutput{}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
				Expect(deletedSubnets).To(ConsistOf("subnet-1", "subnet-2"))
			})

			It("should delete internet gateways when present", func() {
				detachedGWs := []string{}
				deletedGWs := []string{}
				mockEC.DescribeInternetGatewaysFunc = func(ctx context.Context,
					params *ec2.DescribeInternetGatewaysInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
					return &ec2.DescribeInternetGatewaysOutput{
						InternetGateways: []types.InternetGateway{
							{InternetGatewayId: aws.String("igw-1")},
						},
					}, nil
				}
				mockEC.DetachInternetGatewayFunc = func(ctx context.Context,
					params *ec2.DetachInternetGatewayInput,
					optFns ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
					detachedGWs = append(detachedGWs, *params.InternetGatewayId)
					return &ec2.DetachInternetGatewayOutput{}, nil
				}
				mockEC.DeleteInternetGatewayFunc = func(ctx context.Context,
					params *ec2.DeleteInternetGatewayInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
					deletedGWs = append(deletedGWs, *params.InternetGatewayId)
					return &ec2.DeleteInternetGatewayOutput{}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
				Expect(detachedGWs).To(ConsistOf("igw-1"))
				Expect(deletedGWs).To(ConsistOf("igw-1"))
			})

			It("should delete security groups when present", func() {
				deletedSGs := []string{}
				mockEC.DescribeSecurityGroupsFunc = func(ctx context.Context,
					params *ec2.DescribeSecurityGroupsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{
						SecurityGroups: []types.SecurityGroup{
							{GroupId: aws.String("sg-default"), GroupName: aws.String("default")},
							{GroupId: aws.String("sg-custom"), GroupName: aws.String("custom")},
						},
					}, nil
				}
				mockEC.DescribeNetworkInterfacesFunc = func(ctx context.Context,
					params *ec2.DescribeNetworkInterfacesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
					return &ec2.DescribeNetworkInterfacesOutput{}, nil
				}
				mockEC.DeleteSecurityGroupFunc = func(ctx context.Context,
					params *ec2.DeleteSecurityGroupInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
					deletedSGs = append(deletedSGs, *params.GroupId)
					return &ec2.DeleteSecurityGroupOutput{}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
				// Only non-default security groups should be deleted
				Expect(deletedSGs).To(ConsistOf("sg-custom"))
			})

			It("should handle VPC deletion failure with retries", func() {
				deleteAttempts := 0
				mockEC.DeleteVpcFunc = func(ctx context.Context,
					params *ec2.DeleteVpcInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
					deleteAttempts++
					return nil, errors.New("VPC has dependencies")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				// Use a context that won't timeout during retries for this test
				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete VPC"))
				// Should have retried 3 times
				Expect(deleteAttempts).To(Equal(3))
			})
		})

		Describe("CleanupVPC", func() {
			BeforeEach(func() {
				// Setup for full cleanup flow
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					return &ec2.DescribeTagsOutput{Tags: []types.TagDescription{}}, nil
				}
				mockEC.DescribeInstancesFunc = func(ctx context.Context,
					params *ec2.DescribeInstancesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					return &ec2.DescribeInstancesOutput{}, nil
				}
				mockEC.DescribeSecurityGroupsFunc = func(ctx context.Context,
					params *ec2.DescribeSecurityGroupsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{}, nil
				}
				mockEC.DescribeSubnetsFunc = func(ctx context.Context,
					params *ec2.DescribeSubnetsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{}, nil
				}
				mockEC.DescribeRouteTablesFunc = func(ctx context.Context,
					params *ec2.DescribeRouteTablesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
					return &ec2.DescribeRouteTablesOutput{}, nil
				}
				mockEC.DescribeInternetGatewaysFunc = func(ctx context.Context,
					params *ec2.DescribeInternetGatewaysInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
					return &ec2.DescribeInternetGatewaysOutput{}, nil
				}
				mockEC.DeleteVpcFunc = func(ctx context.Context,
					params *ec2.DeleteVpcInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
					return &ec2.DeleteVpcOutput{}, nil
				}
			})

			It("should succeed when no GitHub tags exist", func() {
				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.CleanupVPC(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail when GetTagValue for repository fails", func() {
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					return nil, errors.New("failed to describe tags")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.CleanupVPC(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get GitHubRepository tag"))
			})

			It("should proceed when GITHUB_TOKEN is not set", func() {
				// Clear GITHUB_TOKEN
				originalToken := os.Getenv("GITHUB_TOKEN")
				Expect(os.Unsetenv("GITHUB_TOKEN")).To(Succeed())
				DeferCleanup(func() {
					if originalToken != "" {
						_ = os.Setenv("GITHUB_TOKEN", originalToken) //nolint:errcheck
					}
				})

				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					// Return tags for repository and runID
					for _, filter := range params.Filters {
						if *filter.Name == "key" && len(filter.Values) > 0 {
							if filter.Values[0] == "GitHubRepository" {
								return &ec2.DescribeTagsOutput{
									Tags: []types.TagDescription{
										{Value: aws.String("NVIDIA/holodeck")},
									},
								}, nil
							}
							if filter.Values[0] == "GitHubRunId" {
								return &ec2.DescribeTagsOutput{
									Tags: []types.TagDescription{
										{Value: aws.String("12345")},
									},
								}, nil
							}
						}
					}
					return &ec2.DescribeTagsOutput{}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				// When GITHUB_TOKEN is not set, cleanup should still proceed
				err = cleaner.CleanupVPC(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail when GetTagValue for runID fails", func() {
				callCount := 0
				mockEC.DescribeTagsFunc = func(ctx context.Context,
					params *ec2.DescribeTagsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					callCount++
					// First call for repository succeeds
					if callCount == 1 {
						return &ec2.DescribeTagsOutput{
							Tags: []types.TagDescription{
								{Value: aws.String("NVIDIA/holodeck")},
							},
						}, nil
					}
					// Second call for runID fails
					return nil, errors.New("failed to describe tags")
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.CleanupVPC(context.Background(), "vpc-12345")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get GitHubRunId tag"))
			})
		})

		Describe("deleteRouteTables", func() {
			BeforeEach(func() {
				mockEC.DescribeInstancesFunc = func(ctx context.Context,
					params *ec2.DescribeInstancesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
					return &ec2.DescribeInstancesOutput{}, nil
				}
				mockEC.DescribeSecurityGroupsFunc = func(ctx context.Context,
					params *ec2.DescribeSecurityGroupsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{}, nil
				}
				mockEC.DescribeSubnetsFunc = func(ctx context.Context,
					params *ec2.DescribeSubnetsInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{}, nil
				}
				mockEC.DescribeInternetGatewaysFunc = func(ctx context.Context,
					params *ec2.DescribeInternetGatewaysInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
					return &ec2.DescribeInternetGatewaysOutput{}, nil
				}
				mockEC.DeleteVpcFunc = func(ctx context.Context,
					params *ec2.DeleteVpcInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
					return &ec2.DeleteVpcOutput{}, nil
				}
			})

			It("should replace route table associations before deleting", func() {
				replacedAssocs := []string{}
				deletedRTs := []string{}
				mainRT := true

				mockEC.DescribeRouteTablesFunc = func(ctx context.Context,
					params *ec2.DescribeRouteTablesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
					return &ec2.DescribeRouteTablesOutput{
						RouteTables: []types.RouteTable{
							{
								RouteTableId: aws.String("rtb-main"),
								Associations: []types.RouteTableAssociation{
									{RouteTableAssociationId: aws.String("rtbassoc-main"), Main: &mainRT},
								},
							},
							{
								RouteTableId: aws.String("rtb-custom"),
								Associations: []types.RouteTableAssociation{
									{RouteTableAssociationId: aws.String("rtbassoc-custom")},
								},
							},
						},
					}, nil
				}
				mockEC.ReplaceRouteTableAssociationFunc = func(ctx context.Context,
					params *ec2.ReplaceRouteTableAssociationInput,
					optFns ...func(*ec2.Options)) (*ec2.ReplaceRouteTableAssociationOutput, error) {
					replacedAssocs = append(replacedAssocs, *params.AssociationId)
					return &ec2.ReplaceRouteTableAssociationOutput{}, nil
				}
				mockEC.DeleteRouteTableFunc = func(ctx context.Context,
					params *ec2.DeleteRouteTableInput,
					optFns ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
					deletedRTs = append(deletedRTs, *params.RouteTableId)
					return &ec2.DeleteRouteTableOutput{}, nil
				}

				cleaner, err := New(log, "us-west-2", WithEC2Client(mockEC))
				Expect(err).NotTo(HaveOccurred())

				err = cleaner.DeleteVPCResources(context.Background(), "vpc-12345")
				Expect(err).NotTo(HaveOccurred())
				Expect(replacedAssocs).To(ConsistOf("rtbassoc-custom"))
				Expect(deletedRTs).To(ConsistOf("rtb-custom"))
			})
		})
	})
})
