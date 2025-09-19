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
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
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
				return err
			}

			result, err := a.Run(cmd.Context(), &action.PolicyLintOpts{
				PolicyPath:  policyPath,
				Format:      format,
				RegalConfig: regalConfig,
			})
			if err != nil {
				return err
			}

			if result.Valid {
				logger.Info().Msg("policy is valid!")
				return nil
			}

			if err := output.EncodeOutput(flagOutputFormat, result, policyLintTable); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}
			return fmt.Errorf("%d issues found", len(result.Errors))
		},
	}

	cmd.Flags().StringVarP(&policyPath, "policy", "p", "policy.yaml", "Path to policy file")
	cmd.Flags().BoolVar(&format, "format", false, "Auto-format standalone policy rego files with opa fmt (embedded policies not supported)")
	cmd.Flags().StringVar(&regalConfig, "regal-config", "", "Path to custom regal config (Default: https://github.com/chainloop-dev/chainloop/tree/main/app/cli/internal/policydevel/.regal.yaml)")
	return cmd
}

// Table rendering function for policy lint results
func policyLintTable(result *action.PolicyLintResult) error {
	tw := table.NewWriter()
	tw.SetOutputMirror(os.Stdout)
	tw.AppendHeader(table.Row{"#", "Issue"})

	for i, err := range result.Errors {
		tw.AppendRow(table.Row{i + 1, err})
	}

	tw.Render()
	return nil
}
