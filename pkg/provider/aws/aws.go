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
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
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
	Tags      []types.Tag
	ec2       *ec2.Client
	cacheFile string

	*v1alpha1.Environment
	log *logger.FunLogger
}

func New(log *logger.FunLogger, env v1alpha1.Environment, cacheFile string) (*Provider, error) {
	// Create an AWS session and configure the EC2 client
	region := env.Spec.Region
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

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)
	p := &Provider{
		[]types.Tag{
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
		client,
		cacheFile,
		&env,
		log,
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

func (p *Provider) done() {
	p.log.Done <- struct{}{}
	p.log.Wg.Wait()
}

func (p *Provider) fail() {
	p.log.Fail <- struct{}{}
	p.log.Wg.Wait()
}
