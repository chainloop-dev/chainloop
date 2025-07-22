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

func newPolicyDevelopLintCmd() *cobra.Command {
	var (
		policyPath  string
		format      bool
		regalConfig string
	)

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint chainloop policy structure and content",
		Long: `Performs comprehensive validation of:
- *.yaml files (schema validation)
- *.rego (formatting, linting, structure)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := action.NewPolicyLint(actionOpts)
			if err != nil {
				return fmt.Errorf("failed to initialize linter: %w", err)
			}

			result, err := a.Run(cmd.Context(), &action.PolicyLintOpts{
				PolicyPath:  policyPath,
				Format:      format,
				RegalConfig: regalConfig,
			})
			if err != nil {
				return fmt.Errorf("linting failed: %w", err)
			}

			if result.Valid {
				logger.Info().Msg("policy is valid!")
				return nil
			}

			// Print all validation errors
			for _, err := range result.Errors {
				logger.Error().Msg(err)
			}
			return fmt.Errorf("policy validation failed with %d issues", len(result.Errors))
		},
	}

	cmd.Flags().StringVarP(&policyPath, "policy", "p", ".", "Path to policy directory")
	cmd.Flags().BoolVar(&format, "format", false, "Auto-format file with opa fmt")
	cmd.Flags().StringVar(&regalConfig, "regal-config", "", "Path to custom regal config (Default: https://github.com/chainloop-dev/chainloop/tree/main/app/cli/internal/policydevel/.regal.yaml)")
	return cmd
}
