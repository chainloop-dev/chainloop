//
// Copyright 2025 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newPolicyDevelopInitCmd() *cobra.Command {
	var (
		force       bool
		embedded    bool
		name        string
		description string
		directory   string
		policyType  string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new policy",
		Long: `Initialize a new policy by creating template policy files in the specified directory.

Policy Types:
  rego      - Create a Rego-based policy using Open Policy Agent (default)
  wasm-go   - Create a WebAssembly policy using Go and TinyGo
  wasm-js   - Create a WebAssembly policy using JavaScript and Extism`,
		Example: `  # Initialize a Rego policy in current directory (default)
  chainloop policy develop init

  # Initialize a Rego policy with custom name
  chainloop policy develop init --type rego --name my-policy

  # Initialize a WASM Go policy
  chainloop policy develop init --type wasm-go --name my-policy --directory ./policies

  # Initialize a WASM JS policy
  chainloop policy develop init --type wasm-js --name validation --description "My validation policy"`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if directory == "" {
				directory = "."
			}

			opts := &action.PolicyInitOpts{
				Force:       force,
				Embedded:    embedded,
				Name:        name,
				Description: description,
				Directory:   directory,
				PolicyType:  policyType,
			}

			policyInit, err := action.NewPolicyInit(opts, ActionOpts)
			if err != nil {
				return fmt.Errorf("failed to initialize policy: %w", err)
			}

			if err := policyInit.Run(); err != nil {
				return newGracefulError(err)
			}

			logger.Info().Msg("Initialized policy files")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	cmd.Flags().BoolVar(&embedded, "embedded", false, "initialize an embedded policy (single YAML file, Rego only)")
	cmd.Flags().StringVar(&name, "name", "", "name of the policy")
	cmd.Flags().StringVar(&description, "description", "", "description of the policy")
	cmd.Flags().StringVar(&directory, "directory", "", "directory for policy files")
	cmd.Flags().StringVar(&policyType, "type", "", "policy type: rego (default), wasm-go, or wasm-js")

	return cmd
}
