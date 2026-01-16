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

package ami

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEC2Client is a mock implementation of the EC2ImageDescriber interface.
type mockEC2Client struct {
	describeImagesFunc func(
		ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeImagesOutput, error)
}

// Ensure mockEC2Client implements EC2ImageDescriber.
var _ EC2ImageDescriber = (*mockEC2Client)(nil)

func (m *mockEC2Client) DescribeImages(
	ctx context.Context,
	params *ec2.DescribeImagesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeImagesOutput, error) {
	if m.describeImagesFunc != nil {
		return m.describeImagesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeImagesOutput{}, nil
}

// mockSSMClient is a mock implementation of the SSMParameterGetter interface.
type mockSSMClient struct {
	getParameterFunc func(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)
}

// Ensure mockSSMClient implements SSMParameterGetter.
var _ SSMParameterGetter = (*mockSSMClient)(nil)

func (m *mockSSMClient) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func TestRegistry_Get(t *testing.T) {
	tests := []struct {
		name     string
		osID     string
		wantOK   bool
		wantUser string
	}{
		{
			name:     "ubuntu-22.04 exists",
			osID:     "ubuntu-22.04",
			wantOK:   true,
			wantUser: "ubuntu",
		},
		{
			name:     "ubuntu-24.04 exists",
			osID:     "ubuntu-24.04",
			wantOK:   true,
			wantUser: "ubuntu",
		},
		{
			name:     "amazon-linux-2023 exists",
			osID:     "amazon-linux-2023",
			wantOK:   true,
			wantUser: "ec2-user",
		},
		{
			name:     "rocky-9 exists",
			osID:     "rocky-9",
			wantOK:   true,
			wantUser: "rocky",
		},
		{
			name:   "unknown OS returns false",
			osID:   "unknown-os",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, ok := Get(tt.osID)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.NotNil(t, img)
				assert.Equal(t, tt.wantUser, img.SSHUsername)
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	ids := List()
	assert.NotEmpty(t, ids)
	assert.Contains(t, ids, "ubuntu-22.04")
	assert.Contains(t, ids, "ubuntu-24.04")
	assert.Contains(t, ids, "amazon-linux-2023")
	assert.Contains(t, ids, "rocky-9")

	// Verify sorted order
	for i := 1; i < len(ids); i++ {
		assert.True(t, ids[i-1] < ids[i], "List should be sorted")
	}
}

func TestRegistry_All(t *testing.T) {
	images := All()
	assert.NotEmpty(t, images)
	assert.Len(t, images, len(List()))

	// Verify sorted order
	for i := 1; i < len(images); i++ {
		assert.True(t, images[i-1].ID < images[i].ID, "All should be sorted")
	}
}

func TestRegistry_Exists(t *testing.T) {
	assert.True(t, Exists("ubuntu-22.04"))
	assert.True(t, Exists("amazon-linux-2023"))
	assert.False(t, Exists("unknown-os"))
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"x86_64", "x86_64"},
		{"amd64", "x86_64"},
		{"AMD64", "x86_64"},
		{"arm64", "arm64"},
		{"aarch64", "arm64"},
		{"AARCH64", "arm64"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeArch(tt.input))
		})
	}
}

func TestResolver_Resolve_ViaSSM(t *testing.T) {
	expectedAMI := "ami-ssm-123456"

	ssmClient := &mockSSMClient{
		getParameterFunc: func(
			ctx context.Context,
			params *ssm.GetParameterInput,
			optFns ...func(*ssm.Options),
		) (*ssm.GetParameterOutput, error) {
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Value: aws.String(expectedAMI),
				},
			}, nil
		},
	}

	ec2Client := &mockEC2Client{}
	resolver := NewResolver(ec2Client, ssmClient, "us-west-2")

	result, err := resolver.Resolve(context.Background(), "ubuntu-22.04", "x86_64")
	require.NoError(t, err)
	assert.Equal(t, expectedAMI, result.ImageID)
	assert.Equal(t, "ubuntu", result.SSHUsername)
	assert.Equal(t, OSFamilyDebian, result.OSFamily)
	assert.Equal(t, PackageManagerAPT, result.PackageManager)
}

func TestResolver_Resolve_FallbackToEC2(t *testing.T) {
	expectedAMI := "ami-ec2-789012"

	ssmClient := &mockSSMClient{
		getParameterFunc: func(
			ctx context.Context,
			params *ssm.GetParameterInput,
			optFns ...func(*ssm.Options),
		) (*ssm.GetParameterOutput, error) {
			return nil, errors.New("SSM error")
		},
	}

	ec2Client := &mockEC2Client{
		describeImagesFunc: func(
			ctx context.Context,
			params *ec2.DescribeImagesInput,
			optFns ...func(*ec2.Options),
		) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId:      aws.String(expectedAMI),
						CreationDate: aws.String("2024-01-01T00:00:00.000Z"),
					},
				},
			}, nil
		},
	}

	resolver := NewResolver(ec2Client, ssmClient, "us-west-2")

	result, err := resolver.Resolve(context.Background(), "ubuntu-22.04", "x86_64")
	require.NoError(t, err)
	assert.Equal(t, expectedAMI, result.ImageID)
}

func TestResolver_Resolve_NoSSMPath(t *testing.T) {
	// Rocky Linux has no SSM path, so it should go directly to EC2
	expectedAMI := "ami-rocky-123456"

	ec2Client := &mockEC2Client{
		describeImagesFunc: func(
			ctx context.Context,
			params *ec2.DescribeImagesInput,
			optFns ...func(*ec2.Options),
		) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId:      aws.String(expectedAMI),
						CreationDate: aws.String("2024-01-01T00:00:00.000Z"),
					},
				},
			}, nil
		},
	}

	resolver := NewResolver(ec2Client, nil, "us-west-2")

	result, err := resolver.Resolve(context.Background(), "rocky-9", "x86_64")
	require.NoError(t, err)
	assert.Equal(t, expectedAMI, result.ImageID)
	assert.Equal(t, "rocky", result.SSHUsername)
	assert.Equal(t, OSFamilyRHEL, result.OSFamily)
}

func TestResolver_Resolve_UnknownOS(t *testing.T) {
	resolver := NewResolver(nil, nil, "us-west-2")

	_, err := resolver.Resolve(context.Background(), "unknown-os", "x86_64")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown OS")
	assert.Contains(t, err.Error(), "holodeck os list")
}

func TestResolver_Resolve_UnsupportedArch(t *testing.T) {
	resolver := NewResolver(nil, nil, "us-west-2")

	_, err := resolver.Resolve(context.Background(), "ubuntu-22.04", "ppc64le")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support architecture")
}

func TestResolver_Resolve_ArchNormalization(t *testing.T) {
	expectedAMI := "ami-123456"

	ssmClient := &mockSSMClient{
		getParameterFunc: func(
			ctx context.Context,
			params *ssm.GetParameterInput,
			optFns ...func(*ssm.Options),
		) (*ssm.GetParameterOutput, error) {
			// Verify SSM uses "amd64" for x86_64
			assert.Contains(t, *params.Name, "amd64")
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{
					Value: aws.String(expectedAMI),
				},
			}, nil
		},
	}

	resolver := NewResolver(nil, ssmClient, "us-west-2")

	// Test that "amd64" input gets normalized to "x86_64" for validation
	// but uses "amd64" for SSM path
	result, err := resolver.Resolve(context.Background(), "ubuntu-22.04", "amd64")
	require.NoError(t, err)
	assert.Equal(t, expectedAMI, result.ImageID)
}

func TestResolver_Resolve_SelectsNewestImage(t *testing.T) {
	newestAMI := "ami-newest"
	olderAMI := "ami-older"
	oldestAMI := "ami-oldest"

	ec2Client := &mockEC2Client{
		describeImagesFunc: func(
			ctx context.Context,
			params *ec2.DescribeImagesInput,
			optFns ...func(*ec2.Options),
		) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId:      aws.String(olderAMI),
						CreationDate: aws.String("2024-01-15T00:00:00.000Z"),
					},
					{
						ImageId:      aws.String(newestAMI),
						CreationDate: aws.String("2024-02-01T00:00:00.000Z"),
					},
					{
						ImageId:      aws.String(oldestAMI),
						CreationDate: aws.String("2024-01-01T00:00:00.000Z"),
					},
				},
			}, nil
		},
	}

	resolver := NewResolver(ec2Client, nil, "us-west-2")

	result, err := resolver.Resolve(context.Background(), "rocky-9", "x86_64")
	require.NoError(t, err)
	assert.Equal(t, newestAMI, result.ImageID)
}

func TestResolver_Resolve_NoImagesFound(t *testing.T) {
	ec2Client := &mockEC2Client{
		describeImagesFunc: func(
			ctx context.Context,
			params *ec2.DescribeImagesInput,
			optFns ...func(*ec2.Options),
		) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{},
			}, nil
		},
	}

	resolver := NewResolver(ec2Client, nil, "us-west-2")

	_, err := resolver.Resolve(context.Background(), "rocky-9", "x86_64")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no images found")
}

func TestOSImageMetadata(t *testing.T) {
	// Verify all registered OSes have required fields
	for _, img := range All() {
		t.Run(img.ID, func(t *testing.T) {
			assert.NotEmpty(t, img.ID, "ID required")
			assert.NotEmpty(t, img.Name, "Name required")
			assert.NotEmpty(t, img.Family, "Family required")
			assert.NotEmpty(t, img.SSHUsername, "SSHUsername required")
			assert.NotEmpty(t, img.PackageManager, "PackageManager required")
			assert.Greater(t, img.MinRootVolumeGB, int32(0), "MinRootVolumeGB > 0")
			assert.NotEmpty(t, img.OwnerID, "OwnerID required")
			assert.NotEmpty(t, img.NamePattern, "NamePattern required")
			assert.NotEmpty(t, img.Architectures, "Architectures required")
			assert.Contains(t, img.NamePattern, "%s", "NamePattern needs arch placeholder")
		})
	}
}
