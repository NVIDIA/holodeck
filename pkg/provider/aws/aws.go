/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/ami"
	internalaws "github.com/NVIDIA/holodeck/internal/aws"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	// Name of this builder provider
	Name                             = "aws"
	VpcID                     string = "vpc-id"
	SubnetID                  string = "subnet-id"
	InternetGwID              string = "internet-gateway-id"
	InternetGatewayAttachment string = "internet-gateway-attachment-vpc-id"
	RouteTable                string = "route-table-id"
	SecurityGroupID           string = "security-group-id"
	InstanceID                string = "instance-id"
	PublicDnsName             string = "public-dns-name"
)

var (
	description string = "Holodeck managed AWS Cloud Provider"
)

var (
	yes        = true
	no         = false
	tcp string = "tcp"

	k8s443        int32 = 443
	k8s6443       int32 = 6443
	minMaxCount   int32 = 1
	storageSizeGB int32 = 64
)

type AWS struct {
	Vpcid                     string
	Subnetid                  string
	InternetGwid              string
	InternetGatewayAttachment string
	RouteTable                string
	SecurityGroupid           string
	Instanceid                string
	PublicDnsName             string
}

type Provider struct {
	Tags        []types.Tag
	ec2         internalaws.EC2Client
	ssm         internalaws.SSMClient
	elbv2       internalaws.ELBv2Client
	amiResolver *ami.Resolver
	cacheFile   string

	*v1alpha1.Environment
	log *logger.FunLogger
}

// Option is a functional option for configuring the Provider.
type Option func(*Provider)

// WithEC2Client sets a custom EC2 client for the Provider.
// This is primarily used for testing to inject mock clients.
func WithEC2Client(client internalaws.EC2Client) Option {
	return func(p *Provider) {
		p.ec2 = client
	}
}

// WithSSMClient sets a custom SSM client for the Provider.
// This is primarily used for testing to inject mock clients.
func WithSSMClient(client internalaws.SSMClient) Option {
	return func(p *Provider) {
		p.ssm = client
	}
}

// WithELBv2Client sets a custom ELBv2 client for the Provider.
// This is primarily used for testing to inject mock clients.
func WithELBv2Client(client internalaws.ELBv2Client) Option {
	return func(p *Provider) {
		p.elbv2 = client
	}
}

// WithAMIResolver sets a custom AMI resolver for the Provider.
// This is primarily used for testing.
func WithAMIResolver(resolver *ami.Resolver) Option {
	return func(p *Provider) {
		p.amiResolver = resolver
	}
}

// New creates a new AWS Provider with the given configuration.
// Optional functional options can be provided to customize the provider,
// such as injecting a mock EC2 client for testing.
func New(log *logger.FunLogger, env v1alpha1.Environment, cacheFile string,
	opts ...Option) (*Provider, error) {
	// Create an AWS session and configure the EC2 client
	// For cluster deployments, use cluster region; otherwise use instance region
	var region string
	if env.Spec.Cluster != nil && env.Spec.Cluster.Region != "" {
		region = env.Spec.Cluster.Region
	} else {
		region = env.Spec.Region
	}
	if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
		region = envRegion
	}
	commitSHA := os.Getenv("GITHUB_SHA")
	// short sha
	if len(commitSHA) > 8 {
		commitSHA = commitSHA[:8]
	}
	actor := os.Getenv("GITHUB_ACTOR")
	branch := os.Getenv("GITHUB_REF_NAME")
	repoName := os.Getenv("GITHUB_REPOSITORY")
	gitHubRunId := os.Getenv("GITHUB_RUN_ID")
	gitHubRunNumber := os.Getenv("GITHUB_RUN_NUMBER")
	gitHubJob := os.Getenv("GITHUB_JOB")
	gitHubRunAttempt := os.Getenv("GITHUB_RUN_ATTEMPT")

	p := &Provider{
		Tags: []types.Tag{
			{Key: aws.String("Product"), Value: aws.String("Cloud Native")},
			{Key: aws.String("Name"), Value: aws.String(env.Name)},
			{Key: aws.String("Project"), Value: aws.String("holodeck")},
			{Key: aws.String("Environment"), Value: aws.String("cicd")},
			{Key: aws.String("CommitSHA"), Value: aws.String(commitSHA)},
			{Key: aws.String("Actor"), Value: aws.String(actor)},
			{Key: aws.String("Branch"), Value: aws.String(branch)},
			{Key: aws.String("GitHubRepository"), Value: aws.String(repoName)},
			{Key: aws.String("GitHubRunId"), Value: aws.String(gitHubRunId)},
			{Key: aws.String("GitHubRunNumber"), Value: aws.String(gitHubRunNumber)},
			{Key: aws.String("GitHubJob"), Value: aws.String(gitHubJob)},
			{Key: aws.String("GitHubRunAttempt"), Value: aws.String(gitHubRunAttempt)},
		},
		cacheFile:   cacheFile,
		Environment: &env,
		log:         log,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(p)
	}

	// Create AWS clients if not injected (for testing)
	if p.ec2 == nil || p.ssm == nil {
		cfg, err := config.LoadDefaultConfig(
			context.TODO(),
			config.WithRegion(region),
		)
		if err != nil {
			return nil, err
		}
		if p.ec2 == nil {
			p.ec2 = ec2.NewFromConfig(cfg)
		}
		if p.ssm == nil {
			p.ssm = ssm.NewFromConfig(cfg)
		}
		if p.elbv2 == nil {
			p.elbv2 = elasticloadbalancingv2.NewFromConfig(cfg)
		}
	}

	// Create AMI resolver if not injected
	if p.amiResolver == nil {
		p.amiResolver = ami.NewResolver(p.ec2, p.ssm, region)
	}

	return p, nil
}

// Name returns the name of the builder provisioner
func (p *Provider) Name() string { return Name }

// unmarsalCache unmarshals the cache file into the AWS struct
func (p *Provider) unmarsalCache() (*AWS, error) {
	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](p.cacheFile)
	if err != nil {
		return nil, err
	}

	aws := &AWS{}

	for _, p := range env.Status.Properties {
		switch p.Name {
		case VpcID:
			aws.Vpcid = p.Value
		case SubnetID:
			aws.Subnetid = p.Value
		case InternetGwID:
			aws.InternetGwid = p.Value
		case InternetGatewayAttachment:
			aws.InternetGatewayAttachment = p.Value
		case RouteTable:
			aws.RouteTable = p.Value
		case SecurityGroupID:
			aws.SecurityGroupid = p.Value
		case InstanceID:
			aws.Instanceid = p.Value
		case PublicDnsName:
			aws.PublicDnsName = p.Value
		default:
			// Ignore non AWS infra properties
			continue
		}
	}

	return aws, nil
}

