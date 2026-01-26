/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package validate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"

	cli "github.com/urfave/cli/v2"
)

type command struct {
	log     *logger.FunLogger
	envFile string
	strict  bool
}

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Check   string
	Passed  bool
	Message string
}

// NewCommand constructs the validate command with the specified logger
func NewCommand(log *logger.FunLogger) *cli.Command {
	c := command{
		log: log,
	}
	return c.build()
}

func (m *command) build() *cli.Command {
	validateCmd := cli.Command{
		Name:      "validate",
		Usage:     "Validate a Holodeck environment file",
		ArgsUsage: "",
		Description: `Validate an environment file before creating an instance.

Checks performed:
  - Environment file is valid YAML
  - Required fields are present
  - AWS credentials are configured (for AWS provider)
  - SSH private key is readable
  - SSH public key is readable

Examples:
  # Validate an environment file
  holodeck validate -f env.yaml

  # Strict mode (fail on warnings)
  holodeck validate -f env.yaml --strict`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "envFile",
				Aliases:     []string{"f"},
				Usage:       "Path to the Environment file",
				Destination: &m.envFile,
				Required:    true,
			},
			&cli.BoolFlag{
				Name:        "strict",
				Usage:       "Fail on warnings (not just errors)",
				Destination: &m.strict,
			},
		},
		Action: func(c *cli.Context) error {
			return m.run()
		},
	}

	return &validateCmd
}

func (m *command) run() error {
	results := make([]ValidationResult, 0)
	hasErrors := false
	hasWarnings := false

	// 1. Validate environment file exists and is valid YAML
	env, err := m.validateEnvFile()
	if err != nil {
		results = append(results, ValidationResult{
			Check:   "Environment file",
			Passed:  false,
			Message: err.Error(),
		})
		hasErrors = true
		m.printResults(results)
		return fmt.Errorf("validation failed")
	}
	results = append(results, ValidationResult{
		Check:   "Environment file",
		Passed:  true,
		Message: "Valid YAML structure",
	})

	// 2. Validate required fields
	fieldResults := m.validateRequiredFields(env)
	for _, r := range fieldResults {
		results = append(results, r)
		if !r.Passed {
			hasErrors = true
		}
	}

	// 3. Validate SSH keys
	keyResults := m.validateSSHKeys(env)
	for _, r := range keyResults {
		results = append(results, r)
		if !r.Passed {
			hasErrors = true
		}
	}

	// 4. Validate AWS credentials (if AWS provider)
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		awsResult := m.validateAWSCredentials()
		results = append(results, awsResult)
		if !awsResult.Passed {
			if strings.Contains(awsResult.Message, "warning") {
				hasWarnings = true
			} else {
				hasErrors = true
			}
		}
	}

	// 5. Validate component configuration
	compResults := m.validateComponents(env)
	for _, r := range compResults {
		results = append(results, r)
		if !r.Passed {
			hasWarnings = true
		}
	}

	// Print results
	m.printResults(results)

	// Determine exit status
	if hasErrors {
		return fmt.Errorf("validation failed with errors")
	}
	if hasWarnings && m.strict {
		return fmt.Errorf("validation failed with warnings (strict mode)")
	}

	m.log.Info("\n✅ Validation passed")
	return nil
}

func (m *command) validateEnvFile() (*v1alpha1.Environment, error) {
	if m.envFile == "" {
		return nil, fmt.Errorf("environment file path is required")
	}

	if _, err := os.Stat(m.envFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", m.envFile)
	}

	env, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](m.envFile)
	if err != nil {
		return nil, fmt.Errorf("invalid YAML: %v", err)
	}

	return &env, nil
}

func (m *command) validateRequiredFields(env *v1alpha1.Environment) []ValidationResult {
	results := make([]ValidationResult, 0)

	// Check provider
	if env.Spec.Provider == "" {
		results = append(results, ValidationResult{
			Check:   "Provider",
			Passed:  false,
			Message: "Provider is required (aws or ssh)",
		})
	} else {
		results = append(results, ValidationResult{
			Check:   "Provider",
			Passed:  true,
			Message: fmt.Sprintf("Provider: %s", env.Spec.Provider),
		})
	}

	// Check auth
	if env.Spec.Auth.KeyName == "" {
		results = append(results, ValidationResult{
			Check:   "Auth.KeyName",
			Passed:  false,
			Message: "KeyName is required",
		})
	} else {
		results = append(results, ValidationResult{
			Check:   "Auth.KeyName",
			Passed:  true,
			Message: fmt.Sprintf("KeyName: %s", env.Spec.Auth.KeyName),
		})
	}

	// Check region (for AWS)
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		region := ""
		if env.Spec.Cluster != nil {
			region = env.Spec.Cluster.Region
		} else {
			region = env.Spec.Instance.Region
		}

		if region == "" {
			results = append(results, ValidationResult{
				Check:   "Region",
				Passed:  false,
				Message: "Region is required for AWS provider",
			})
		} else {
			results = append(results, ValidationResult{
				Check:   "Region",
				Passed:  true,
				Message: fmt.Sprintf("Region: %s", region),
			})
		}
	}

	// Check instance type or cluster config
	if env.Spec.Provider == v1alpha1.ProviderAWS {
		if env.Spec.Cluster == nil {
			if env.Spec.Instance.Type == "" {
				results = append(results, ValidationResult{
					Check:   "Instance.Type",
					Passed:  false,
					Message: "Instance type is required for single-node AWS deployment",
				})
			} else {
				results = append(results, ValidationResult{
					Check:   "Instance.Type",
					Passed:  true,
					Message: fmt.Sprintf("Instance type: %s", env.Spec.Instance.Type),
				})
			}
		} else {
			results = append(results, ValidationResult{
				Check:   "Cluster config",
				Passed:  true,
				Message: fmt.Sprintf("Cluster mode: %d CP, %d workers",
					env.Spec.Cluster.ControlPlane.Count,
					func() int32 {
						if env.Spec.Cluster.Workers != nil {
							return env.Spec.Cluster.Workers.Count
						}
						return 0
					}()),
			})
		}
	}

	// Check host URL for SSH provider
	if env.Spec.Provider == v1alpha1.ProviderSSH {
		if env.Spec.Instance.HostUrl == "" {
			results = append(results, ValidationResult{
				Check:   "HostUrl",
				Passed:  false,
				Message: "HostUrl is required for SSH provider",
			})
		} else {
			results = append(results, ValidationResult{
				Check:   "HostUrl",
				Passed:  true,
				Message: fmt.Sprintf("Host: %s", env.Spec.Instance.HostUrl),
			})
		}
	}

	return results
}

func (m *command) validateSSHKeys(env *v1alpha1.Environment) []ValidationResult {
	results := make([]ValidationResult, 0)

	// Check private key
	if env.Spec.Auth.PrivateKey == "" {
		results = append(results, ValidationResult{
			Check:   "SSH private key",
			Passed:  false,
			Message: "Private key path is required",
		})
	} else {
		// Expand home directory
		keyPath := expandPath(env.Spec.Auth.PrivateKey)
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			results = append(results, ValidationResult{
				Check:   "SSH private key",
				Passed:  false,
				Message: fmt.Sprintf("Private key not found: %s", keyPath),
			})
		} else {
			// Check if readable
			if _, err := os.ReadFile(keyPath); err != nil {
				results = append(results, ValidationResult{
					Check:   "SSH private key",
					Passed:  false,
					Message: fmt.Sprintf("Cannot read private key: %v", err),
				})
			} else {
				results = append(results, ValidationResult{
					Check:   "SSH private key",
					Passed:  true,
					Message: fmt.Sprintf("Readable: %s", keyPath),
				})
			}
		}
	}

	// Check public key
	if env.Spec.Auth.PublicKey == "" {
		results = append(results, ValidationResult{
			Check:   "SSH public key",
			Passed:  false,
			Message: "Public key path is required",
		})
	} else {
		keyPath := expandPath(env.Spec.Auth.PublicKey)
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			results = append(results, ValidationResult{
				Check:   "SSH public key",
				Passed:  false,
				Message: fmt.Sprintf("Public key not found: %s", keyPath),
			})
		} else {
			results = append(results, ValidationResult{
				Check:   "SSH public key",
				Passed:  true,
				Message: fmt.Sprintf("Found: %s", keyPath),
			})
		}
	}

	return results
}

func (m *command) validateAWSCredentials() ValidationResult {
	// Try to load AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ValidationResult{
			Check:   "AWS credentials",
			Passed:  false,
			Message: fmt.Sprintf("Failed to load AWS config: %v", err),
		}
	}

	// Check if credentials are available
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return ValidationResult{
			Check:   "AWS credentials",
			Passed:  false,
			Message: fmt.Sprintf("Failed to retrieve credentials: %v", err),
		}
	}

	if creds.AccessKeyID == "" {
		return ValidationResult{
			Check:   "AWS credentials",
			Passed:  false,
			Message: "No AWS access key found",
		}
	}

	return ValidationResult{
		Check:   "AWS credentials",
		Passed:  true,
		Message: fmt.Sprintf("Configured (source: %s)", creds.Source),
	}
}

func (m *command) validateComponents(env *v1alpha1.Environment) []ValidationResult {
	results := make([]ValidationResult, 0)

	// Check for common misconfigurations
	if env.Spec.NVIDIAContainerToolkit.Install && !env.Spec.ContainerRuntime.Install {
		results = append(results, ValidationResult{
			Check:   "Component dependencies",
			Passed:  false,
			Message: "Warning: Container Toolkit requires a container runtime",
		})
	}

	if env.Spec.Kubernetes.Install && !env.Spec.ContainerRuntime.Install {
		results = append(results, ValidationResult{
			Check:   "Component dependencies",
			Passed:  false,
			Message: "Warning: Kubernetes requires a container runtime",
		})
	}

	// Check driver branch/version
	if env.Spec.NVIDIADriver.Install {
		if env.Spec.NVIDIADriver.Version != "" && env.Spec.NVIDIADriver.Branch != "" {
			results = append(results, ValidationResult{
				Check:   "NVIDIA Driver config",
				Passed:  true,
				Message: "Both version and branch specified; version takes precedence",
			})
		}
	}

	// Kubernetes installer validation
	if env.Spec.Kubernetes.Install {
		installer := env.Spec.Kubernetes.KubernetesInstaller
		if installer == "" {
			installer = "kubeadm"
		}
		validInstallers := map[string]bool{"kubeadm": true, "kind": true, "microk8s": true}
		if !validInstallers[installer] {
			results = append(results, ValidationResult{
				Check:   "Kubernetes installer",
				Passed:  false,
				Message: fmt.Sprintf("Warning: Unknown installer %q, expected kubeadm/kind/microk8s", installer),
			})
		}
	}

	return results
}

func (m *command) printResults(results []ValidationResult) {
	fmt.Println("\n=== Validation Results ===\n")

	for _, r := range results {
		icon := "✓"
		if !r.Passed {
			icon = "✗"
		}
		fmt.Printf("  %s %s\n", icon, r.Check)
		fmt.Printf("    %s\n", r.Message)
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
