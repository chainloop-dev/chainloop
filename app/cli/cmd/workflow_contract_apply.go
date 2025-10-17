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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newWorkflowContractApplyCmd() *cobra.Command {
	var filePath, name, description, projectName string
	var contractName string
	var rawContract []byte

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a contract (create or update)",
		Long: `Apply a contract from a file. This command will create the contract if it doesn't exist,
or update it if it already exists.`,
		Example: `  # Apply a contract from file
  chainloop workflow contract apply --contract my-contract.yaml

  # Apply to a specific project
  chainloop workflow contract apply --contract my-contract.yaml --project my-project`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			contractName = name

			if filePath != "" {
				var err error
				rawContract, err = action.LoadFileOrURL(filePath)
				if err != nil {
					return fmt.Errorf("failed to read contract file: %w", err)
				}

				// Extract name from the contract file content
				extractedName, err := action.ExtractNameFromRawSchema(rawContract)
				if err != nil {
					return err
				}

				// For v2 schemas, use the extracted name. For v1 schemas, extractedName will be empty
				if extractedName == "" && name == "" {
					return fmt.Errorf("contracts require --name flag to specify the contract name")
				} else if extractedName != "" {
					contractName = extractedName
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			var desc *string
			if cmd.Flags().Changed("description") {
				desc = &description
			}

			res, err := action.NewWorkflowContractApply(ActionOpts).Run(cmd.Context(), rawContract, contractName, desc, projectName)
			if err != nil {
				return err
			}

			logger.Info().Msg("Contract applied!")
			return output.EncodeOutput(flagOutputFormat, res, contractItemTableOutput)
		},
	}

	cmd.Flags().StringVarP(&filePath, "contract", "f", "", "workflow contract file path (optional)")
	cmd.Flags().StringVar(&name, "name", "", "contract name (required if no contract file provided)")
	cmd.Flags().StringVar(&description, "description", "", "contract description")
	cmd.Flags().StringVar(&projectName, "project", "", "project name to scope the contract")

	return cmd
}
