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

package delete

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provider/aws"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

type options struct {
	cachePath string
	envFile   string

	cachefile string

	cfg   v1alpha1.Environment
	cache v1alpha1.Environment
}

type command struct {
	logger *logrus.Logger
}

// NewCommand constructs the delete command with the specified logger
func NewCommand(logger *logrus.Logger) *cli.Command {
	c := command{
		logger: logger,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	opts := options{}

	// Create the 'delete' command
	create := cli.Command{
		Name:  "delete",
		Usage: "delete a test environment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cachepath",
				Aliases:     []string{"c"},
				Usage:       "Path to the cache directory",
				Destination: &opts.cachePath,
			},
			&cli.StringFlag{
				Name:        "envFile",
				Aliases:     []string{"f"},
				Usage:       "Path to the Environment file",
				Destination: &opts.envFile,
			},
		},
		Before: func(c *cli.Context) error {
			// Read the config file
			var err error
			opts.cfg, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.envFile)
			if err != nil {
				fmt.Printf("Error reading config file: %s\n", err)
				return err
			}

			if opts.cfg.Spec.Provider != v1alpha1.ProviderAWS {
				fmt.Printf("Only AWS provider is supported\n")
				return err
			}

			// read hostUrl from cache
			if opts.cachePath == "" {
				opts.cachePath = filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
			}
			opts.cachefile = filepath.Join(opts.cachePath, opts.cfg.Name+".yaml")
			opts.cache, err = jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.cachefile)
			if err != nil {
				return err
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			return m.run(c, &opts)
		},
	}

	return &create
}

func (m command) run(c *cli.Context, opts *options) error {
	if opts.cachePath == "" {
		opts.cachePath = filepath.Join(os.Getenv("HOME"), ".cache", "holodeck")
	}

	cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](opts.envFile)
	if err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}
	cachefile := filepath.Join(opts.cachePath, cfg.Name+".yaml")

	client, err := aws.New(cfg, cachefile)
	if err != nil {
		fmt.Printf("Error creating client: %s\n", err)
		os.Exit(1)
	}

	// check if cache exists
	if _, err := os.Stat(cachefile); err != nil {
		fmt.Printf("Error reading cache file: %s\n", err)
		fmt.Printf("Cache file %s does not exist\n", cachefile)
		os.Exit(1)
	}

	err = client.Delete()
	if err != nil {
		fmt.Printf("Error deleting environment: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted environment %s\n", cfg.Name)

	return nil
}
