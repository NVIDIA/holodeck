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
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/ami"
	"github.com/NVIDIA/holodeck/internal/logger"
)

// mockSSMClient implements ami.SSMParameterGetter for testing.
type mockSSMClient struct {
	GetParameterFunc func(ctx context.Context, params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

func (m *mockSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.GetParameterFunc != nil {
		return m.GetParameterFunc(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Value: aws.String("ami-ssm-resolved"),
		},
	}, nil
}

func TestResolveImageForNode(t *testing.T) {
	tests := []struct {
		name           string
		os             string
		image          *v1alpha1.Image
		arch           string
		instanceOS     string // p.Spec.Instance.OS fallback
		setupMock      func(*MockEC2Client, *mockSSMClient)
		wantImageID    string
		wantSSHUser    string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "explicit ImageId takes precedence",
			os:   "ubuntu-22.04", // Should be ignored
			image: &v1alpha1.Image{
				ImageId:      aws.String("ami-explicit-123"),
				Architecture: "x86_64",
			},
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ec2Mock.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-explicit-123"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}
			},
			wantImageID: "ami-explicit-123",
			wantSSHUser: "", // Must be provided in auth config
			wantErr:     false,
		},
		{
			name:  "OS specified resolves via AMI resolver (SSM)",
			os:    "ubuntu-22.04",
			image: &v1alpha1.Image{Architecture: "x86_64"},
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &ssmtypes.Parameter{
							Value: aws.String("ami-ubuntu-2204-ssm"),
						},
					}, nil
				}
			},
			wantImageID: "ami-ubuntu-2204-ssm",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:  "OS specified resolves via DescribeImages fallback",
			os:    "rocky-9", // Rocky has no SSM path
			image: &v1alpha1.Image{Architecture: "x86_64"},
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ec2Mock.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-rocky9-latest"),
								CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
			},
			wantImageID: "ami-rocky9-latest",
			wantSSHUser: "rocky",
			wantErr:     false,
		},
		{
			name:       "falls back to Instance.OS when node OS not specified",
			os:         "", // No node-specific OS
			image:      nil,
			instanceOS: "ubuntu-24.04",
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &ssmtypes.Parameter{
							Value: aws.String("ami-ubuntu-2404-fallback"),
						},
					}, nil
				}
			},
			wantImageID: "ami-ubuntu-2404-fallback",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:       "falls back to legacy Ubuntu 22.04 when no OS specified",
			os:         "",
			image:      nil,
			instanceOS: "", // No Instance.OS either
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ec2Mock.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-legacy-ubuntu"),
								CreationDate: aws.String("2026-01-15T00:00:00.000Z"),
							},
							{
								ImageId:      aws.String("ami-legacy-ubuntu-older"),
								CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
			},
			wantImageID: "ami-legacy-ubuntu", // Newest by creation date
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:  "architecture from image parameter",
			os:    "ubuntu-22.04",
			image: &v1alpha1.Image{Architecture: "arm64"},
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					// Verify arm64 is in the SSM path
					if params.Name != nil && strings.Contains(*params.Name, "arm64") {
						return &ssm.GetParameterOutput{
							Parameter: &ssmtypes.Parameter{
								Value: aws.String("ami-ubuntu-arm64"),
							},
						}, nil
					}
					return nil, fmt.Errorf("unexpected SSM path: %s", *params.Name)
				}
			},
			wantImageID: "ami-ubuntu-arm64",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:  "architecture from arch parameter overrides image",
			os:    "ubuntu-22.04",
			image: &v1alpha1.Image{Architecture: "x86_64"}, // Should be overridden
			arch:  "arm64",
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &ssmtypes.Parameter{
							Value: aws.String("ami-ubuntu-arm64-override"),
						},
					}, nil
				}
			},
			wantImageID: "ami-ubuntu-arm64-override",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:  "defaults to x86_64 when no architecture specified",
			os:    "ubuntu-22.04",
			image: nil,
			arch:  "",
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					// SSM uses "amd64" for x86_64
					if params.Name != nil && strings.Contains(*params.Name, "amd64") {
						return &ssm.GetParameterOutput{
							Parameter: &ssmtypes.Parameter{
								Value: aws.String("ami-ubuntu-x86"),
							},
						}, nil
					}
					return nil, fmt.Errorf("expected amd64 in path, got: %s", *params.Name)
				}
			},
			wantImageID: "ami-ubuntu-x86",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
		{
			name:           "error on unknown OS",
			os:             "unknown-os-999",
			image:          nil,
			wantErr:        true,
			wantErrContain: "unknown OS",
		},
		{
			name:  "error on unsupported architecture",
			os:    "ubuntu-22.04",
			image: nil,
			arch:  "ppc64le", // Not supported
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				// Should not reach SSM
			},
			wantErr:        true,
			wantErrContain: "does not support architecture",
		},
		{
			name:       "error on legacy AMI lookup failure",
			os:         "",
			image:      nil,
			instanceOS: "",
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ec2Mock.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("EC2 API error")
				}
			},
			wantErr:        true,
			wantErrContain: "failed to describe images",
		},
		{
			name: "empty ImageId string is not treated as explicit",
			os:   "ubuntu-22.04",
			image: &v1alpha1.Image{
				ImageId:      aws.String(""), // Empty string
				Architecture: "x86_64",
			},
			setupMock: func(ec2Mock *MockEC2Client, ssmMock *mockSSMClient) {
				ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
					optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &ssmtypes.Parameter{
							Value: aws.String("ami-from-os-field"),
						},
					}, nil
				}
			},
			wantImageID: "ami-from-os-field",
			wantSSHUser: "ubuntu",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			ec2Mock := NewMockEC2Client()
			ssmMock := &mockSSMClient{}

			if tt.setupMock != nil {
				tt.setupMock(ec2Mock, ssmMock)
			}

			// Create AMI resolver with mocks
			resolver := ami.NewResolver(ec2Mock, ssmMock, "us-east-1")

			// Create provider with minimal config
			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
						OS:     tt.instanceOS,
					},
				},
			}

			p := &Provider{
				Environment: &env,
				ec2:         ec2Mock,
				amiResolver: resolver,
			}

			// Call the function under test
			result, err := p.resolveImageForNode(tt.os, tt.image, tt.arch)

			// Assertions
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantImageID, result.ImageID)
			assert.Equal(t, tt.wantSSHUser, result.SSHUsername)
			// Architecture should always be set on successful resolution
			assert.NotEmpty(t, result.Architecture)
		})
	}
}

func TestResolveImageForNode_DoesNotMutateState(t *testing.T) {
	// This test verifies the P0 fix: resolveImageForNode should not mutate
	// provider state, making it safe for cluster mode with heterogeneous nodes.

	ec2Mock := NewMockEC2Client()
	ec2Mock.DescribeImagesFunc = func(ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      aws.String("ami-legacy-fallback"),
					CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
				},
			},
		}, nil
	}

	ssmMock := &mockSSMClient{}
	resolver := ami.NewResolver(ec2Mock, ssmMock, "us-east-1")

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "t3.medium",
				Region: "us-east-1",
				Image:  v1alpha1.Image{Architecture: "x86_64"},
			},
		},
	}

	p := &Provider{
		Environment: &env,
		ec2:         ec2Mock,
		amiResolver: resolver,
	}

	// Capture initial state
	initialImageId := p.Spec.Image.ImageId

	// Call resolveImageForNode with no OS (triggers legacy fallback)
	result, err := p.resolveImageForNode("", nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ami-legacy-fallback", result.ImageID)

	// Verify provider state was NOT mutated
	assert.Equal(t, initialImageId, p.Spec.Image.ImageId,
		"resolveImageForNode should not mutate p.Spec.Image.ImageId")
}

func TestFindLegacyAMI(t *testing.T) {
	tests := []struct {
		name           string
		arch           string
		imageArch      string // p.Spec.Image.Architecture
		setupMock      func(*MockEC2Client)
		wantImageID    string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "finds newest Ubuntu 22.04 x86_64",
			arch: "x86_64",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-older"),
								CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
							},
							{
								ImageId:      aws.String("ami-newest"),
								CreationDate: aws.String("2026-01-15T00:00:00.000Z"),
							},
							{
								ImageId:      aws.String("ami-middle"),
								CreationDate: aws.String("2026-01-10T00:00:00.000Z"),
							},
						},
					}, nil
				}
			},
			wantImageID: "ami-newest",
			wantErr:     false,
		},
		{
			name: "finds arm64 AMI",
			arch: "arm64",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					// Verify arm64 filter is passed
					for _, f := range params.Filters {
						if *f.Name == "architecture" && f.Values[0] == "arm64" {
							return &ec2.DescribeImagesOutput{
								Images: []types.Image{
									{
										ImageId:      aws.String("ami-arm64"),
										CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
									},
								},
							}, nil
						}
					}
					return nil, fmt.Errorf("expected arm64 architecture filter")
				}
			},
			wantImageID: "ami-arm64",
			wantErr:     false,
		},
		{
			name:      "uses provider architecture when arch param empty",
			arch:      "",
			imageArch: "arm64",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-from-provider-arch"),
								CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
							},
						},
					}, nil
				}
			},
			wantImageID: "ami-from-provider-arch",
			wantErr:     false,
		},
		{
			name: "returns error when no images found",
			arch: "x86_64",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("no images found")
				}
			},
			wantErr:        true,
			wantErrContain: "no images found",
		},
		{
			name: "returns error on EC2 API failure",
			arch: "x86_64",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("EC2 API unavailable")
				}
			},
			wantErr:        true,
			wantErrContain: "failed to describe images",
		},
		{
			name:           "returns error on invalid architecture",
			arch:           "invalid-arch",
			wantErr:        true,
			wantErrContain: "invalid architecture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Mock := NewMockEC2Client()
			if tt.setupMock != nil {
				tt.setupMock(ec2Mock)
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   "t3.medium",
						Region: "us-east-1",
						Image:  v1alpha1.Image{Architecture: tt.imageArch},
					},
				},
			}

			p := &Provider{
				Environment: &env,
				ec2:         ec2Mock,
			}

			imageID, err := p.findLegacyAMI(tt.arch)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantImageID, imageID)
		})
	}
}

func TestDescribeImageArch(t *testing.T) {
	tests := []struct {
		name           string
		imageID        string
		setupMock      func(*MockEC2Client)
		wantArch       string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:    "returns x86_64 architecture",
			imageID: "ami-x86-123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-x86-123"),
								Architecture: types.ArchitectureValuesX8664,
							},
						},
					}, nil
				}
			},
			wantArch: "x86_64",
			wantErr:  false,
		},
		{
			name:    "returns arm64 architecture",
			imageID: "ami-arm-456",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{
							{
								ImageId:      aws.String("ami-arm-456"),
								Architecture: types.ArchitectureValuesArm64,
							},
						},
					}, nil
				}
			},
			wantArch: "arm64",
			wantErr:  false,
		},
		{
			name:    "error when image not found",
			imageID: "ami-missing",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []types.Image{},
					}, nil
				}
			},
			wantErr:        true,
			wantErrContain: "not found",
		},
		{
			name:    "error on EC2 API failure",
			imageID: "ami-fail",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(ctx context.Context,
					params *ec2.DescribeImagesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("EC2 API error")
				}
			},
			wantErr:        true,
			wantErrContain: "failed to describe image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Mock := NewMockEC2Client()
			if tt.setupMock != nil {
				tt.setupMock(ec2Mock)
			}

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

			p := &Provider{
				Environment: &env,
				ec2:         ec2Mock,
			}

			arch, err := p.describeImageArch(tt.imageID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantArch, arch)
		})
	}
}

func TestGetInstanceTypeArch(t *testing.T) {
	tests := []struct {
		name           string
		instanceType   string
		setupMock      func(*MockEC2Client)
		wantArchs      []string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "returns x86_64 for t3.medium",
			instanceType: "t3.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: types.InstanceTypeT3Medium,
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeX8664,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArchs: []string{"x86_64"},
			wantErr:   false,
		},
		{
			name:         "returns arm64 for t4g.medium",
			instanceType: "t4g.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: types.InstanceTypeT4gMedium,
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeArm64,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArchs: []string{"arm64"},
			wantErr:   false,
		},
		{
			name:         "error when instance type not found",
			instanceType: "t99.nonexistent",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{},
					}, nil
				}
			},
			wantErr:        true,
			wantErrContain: "not found",
		},
		{
			name:         "error on EC2 API failure",
			instanceType: "t3.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return nil, fmt.Errorf("EC2 API error")
				}
			},
			wantErr:        true,
			wantErrContain: "failed to describe instance type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Mock := NewMockEC2Client()
			if tt.setupMock != nil {
				tt.setupMock(ec2Mock)
			}

			env := v1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
				Spec: v1alpha1.EnvironmentSpec{
					Provider: v1alpha1.ProviderAWS,
					Instance: v1alpha1.Instance{
						Type:   tt.instanceType,
						Region: "us-east-1",
					},
				},
			}

			p := &Provider{
				Environment: &env,
				ec2:         ec2Mock,
			}

			archs, err := p.getInstanceTypeArch(tt.instanceType)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantArchs, archs)
		})
	}
}

func TestResolveImageForNode_ExplicitImageId_ReturnsArchitecture(t *testing.T) {
	// Verify that when an explicit ImageId is provided, the architecture
	// is queried from EC2 and returned in the result.
	ec2Mock := NewMockEC2Client()
	ec2Mock.DescribeImagesFunc = func(ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		// Verify the correct AMI is being queried
		assert.Equal(t, []string{"ami-arm64-custom"}, params.ImageIds)
		return &ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      aws.String("ami-arm64-custom"),
					Architecture: types.ArchitectureValuesArm64,
				},
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
			},
		},
	}

	p := &Provider{
		Environment: &env,
		ec2:         ec2Mock,
	}

	image := &v1alpha1.Image{
		ImageId: aws.String("ami-arm64-custom"),
	}
	result, err := p.resolveImageForNode("", image, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ami-arm64-custom", result.ImageID)
	assert.Equal(t, "arm64", result.Architecture)
	assert.Equal(t, "", result.SSHUsername) // Must be provided in auth config
}

func TestDryRun_ArchitectureMismatch(t *testing.T) {
	// Test that DryRun detects when an arm64 AMI is used with an x86_64 instance type.
	ec2Mock := NewMockEC2Client()

	// checkInstanceTypes needs to find the instance type
	ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
		params *ec2.DescribeInstanceTypesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
		// If called with specific InstanceTypes (getInstanceTypeArch), return processor info
		if len(params.InstanceTypes) > 0 {
			return &ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{
					{
						InstanceType: types.InstanceTypeT3Medium,
						ProcessorInfo: &types.ProcessorInfo{
							SupportedArchitectures: []types.ArchitectureType{
								types.ArchitectureTypeX8664,
							},
						},
					},
				},
			}, nil
		}
		// Paginated scan for checkInstanceTypes
		return &ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []types.InstanceTypeInfo{
				{
					InstanceType: types.InstanceTypeT3Medium,
					ProcessorInfo: &types.ProcessorInfo{
						SupportedArchitectures: []types.ArchitectureType{
							types.ArchitectureTypeX8664,
						},
					},
				},
			},
		}, nil
	}

	// checkImages -> assertImageIdSupported needs to find the image
	ec2Mock.DescribeImagesFunc = func(ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      aws.String("ami-arm64-image"),
					CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
					Architecture: types.ArchitectureValuesArm64,
				},
			},
		}, nil
	}

	log := logger.NewLogger()

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "t3.medium", // x86_64 only
				Region: "us-east-1",
				Image: v1alpha1.Image{
					ImageId:      aws.String("ami-arm64-image"),
					Architecture: "arm64", // Mismatched!
				},
			},
			Auth: v1alpha1.Auth{
				KeyName: "test-key",
			},
		},
	}

	p := &Provider{
		Environment: &env,
		ec2:         ec2Mock,
		log:         log,
	}

	err := p.DryRun()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "architecture mismatch")
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "t3.medium")
}

func TestDryRun_ArchitectureMatch(t *testing.T) {
	// Test that DryRun succeeds when architecture matches.
	ec2Mock := NewMockEC2Client()

	ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
		params *ec2.DescribeInstanceTypesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
		return &ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []types.InstanceTypeInfo{
				{
					InstanceType: types.InstanceTypeT4gMedium,
					ProcessorInfo: &types.ProcessorInfo{
						SupportedArchitectures: []types.ArchitectureType{
							types.ArchitectureTypeArm64,
						},
					},
				},
			},
		}, nil
	}

	ec2Mock.DescribeImagesFunc = func(ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []types.Image{
				{
					ImageId:      aws.String("ami-arm64-image"),
					CreationDate: aws.String("2026-01-01T00:00:00.000Z"),
					Architecture: types.ArchitectureValuesArm64,
				},
			},
		}, nil
	}

	log := logger.NewLogger()

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-env"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "t4g.medium", // arm64
				Region: "us-east-1",
				Image: v1alpha1.Image{
					ImageId:      aws.String("ami-arm64-image"),
					Architecture: "arm64", // Matches!
				},
			},
			Auth: v1alpha1.Auth{
				KeyName: "test-key",
			},
		},
	}

	p := &Provider{
		Environment: &env,
		ec2:         ec2Mock,
		log:         log,
	}

	err := p.DryRun()
	require.NoError(t, err)
}

func TestInferArchFromInstanceType(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		setupMock    func(*MockEC2Client)
		wantArch     string
		wantErr      bool
	}{
		{
			name:         "arm64-only instance type infers arm64",
			instanceType: "g5g.xlarge",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: "g5g.xlarge",
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeArm64,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArch: "arm64",
			wantErr:  false,
		},
		{
			name:         "x86_64-only instance type infers x86_64",
			instanceType: "g4dn.xlarge",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: "g4dn.xlarge",
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeX8664,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArch: "x86_64",
			wantErr:  false,
		},
		{
			name:         "dual-arch instance type defaults to x86_64",
			instanceType: "synthetic.dualarch",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: "synthetic.dualarch",
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeX8664,
										types.ArchitectureTypeArm64,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArch: "x86_64",
			wantErr:  false,
		},
		{
			name:         "arm64_mac variant infers arm64",
			instanceType: "mac2-m2.metal",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: "mac2-m2.metal",
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeArm64Mac,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArch: "arm64",
			wantErr:  false,
		},
		{
			name:         "x86_64_mac variant infers x86_64",
			instanceType: "mac1.metal",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []types.InstanceTypeInfo{
							{
								InstanceType: "mac1.metal",
								ProcessorInfo: &types.ProcessorInfo{
									SupportedArchitectures: []types.ArchitectureType{
										types.ArchitectureTypeX8664Mac,
									},
								},
							},
						},
					}, nil
				}
			},
			wantArch: "x86_64",
			wantErr:  false,
		},
		{
			name:         "API error returns error",
			instanceType: "unknown.type",
			setupMock: func(ec2Mock *MockEC2Client) {
				ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
					params *ec2.DescribeInstanceTypesInput,
					optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return nil, fmt.Errorf("instance type not found")
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Mock := NewMockEC2Client()
			if tt.setupMock != nil {
				tt.setupMock(ec2Mock)
			}

			p := &Provider{ec2: ec2Mock}
			arch, err := p.inferArchFromInstanceType(tt.instanceType)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantArch, arch)
		})
	}
}

func TestResolveOSToAMI_InfersArchFromInstanceType(t *testing.T) {
	// When Architecture is empty and instance type is arm64-only,
	// resolveOSToAMI should infer arm64 and resolve an arm64 AMI.
	ec2Mock := NewMockEC2Client()
	ssmMock := &mockSSMClient{}

	// Mock: g5g.xlarge is arm64-only
	ec2Mock.DescribeInstTypesFunc = func(ctx context.Context,
		params *ec2.DescribeInstanceTypesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
		return &ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []types.InstanceTypeInfo{
				{
					InstanceType: "g5g.xlarge",
					ProcessorInfo: &types.ProcessorInfo{
						SupportedArchitectures: []types.ArchitectureType{
							types.ArchitectureTypeArm64,
						},
					},
				},
			},
		}, nil
	}

	// Mock: SSM returns arm64 AMI when arm64 is in path
	ssmMock.GetParameterFunc = func(ctx context.Context, params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
		if params.Name != nil && strings.Contains(*params.Name, "arm64") {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Value: aws.String("ami-arm64-inferred"),
				},
			}, nil
		}
		return nil, fmt.Errorf("expected arm64 in SSM path, got: %s", *params.Name)
	}

	resolver := ami.NewResolver(ec2Mock, ssmMock, "us-east-1")

	env := v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-arm64-inference"},
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
			Instance: v1alpha1.Instance{
				Type:   "g5g.xlarge", // arm64-only instance type
				Region: "us-east-1",
				OS:     "ubuntu-22.04",
			},
			// Architecture is intentionally NOT set
		},
	}

	p := &Provider{
		Environment: &env,
		ec2:         ec2Mock,
		amiResolver: resolver,
	}

	err := p.resolveOSToAMI()
	require.NoError(t, err)

	// Architecture should have been inferred as arm64
	assert.Equal(t, "arm64", p.Spec.Image.Architecture,
		"Should infer arm64 from g5g.xlarge instance type")
	// AMI should be the arm64 one
	require.NotNil(t, p.Spec.Image.ImageId)
	assert.Equal(t, "ami-arm64-inferred", *p.Spec.Image.ImageId,
		"Should resolve arm64 AMI when architecture inferred from instance type")
}
