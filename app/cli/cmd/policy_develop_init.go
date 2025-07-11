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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newPolicyDevelopInitCmd() *cobra.Command {
	var (
		force       bool
		embedded    bool
		name        string
		description string
	)

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new policy",
		Long: `Initialize a new policy by creating template policy files in the specified directory.
By default, it creates chainloop-policy.yaml and chainloop-policy.rego files.`,
		Example: `  
  # Initialize in current directory with separate files
  chainloop policy develop init

  # Initialize in specific directory with embedded format
  chainloop policy develop init ./policies --embedded`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to current directory if not specified
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			opts := &action.PolicyInitOpts{
				Force:       force,
				Embedded:    embedded,
				Name:        name,
				Description: description,
			}

			policyInit, err := action.NewPolicyInit(opts, actionOpts)
			if err != nil {
				return fmt.Errorf("failed to initialize policy: %w", err)
			}

			if err := policyInit.Run(cmd.Context(), dir); err != nil {
				return newGracefulError(err)
			}

			logger.Info().Msg("Initialized policy files")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	cmd.Flags().BoolVar(&embedded, "embedded", false, "initialize an embedded policy (single YAML file)")
	cmd.Flags().StringVar(&name, "name", "", "name of the policy")
	cmd.Flags().StringVar(&description, "description", "", "description of the policy")

	return cmd
}
