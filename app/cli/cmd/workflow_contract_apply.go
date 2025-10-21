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
	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newWorkflowContractApplyCmd() *cobra.Command {
	var contractPath, name, description, projectName string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a contract (create or update)",
		Long: `Apply a contract from a file. This command will create the contract if it doesn't exist,
or update it if it already exists.`,
		Example: `  # Apply a contract from file
  chainloop workflow contract apply --contract my-contract.yaml --name my-contract --project my-project`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var desc *string
			if cmd.Flags().Changed("description") {
				desc = &description
			}

			res, err := action.NewWorkflowContractApply(ActionOpts).Run(cmd.Context(), name, contractPath, desc, projectName)
			if err != nil {
				return err
			}

			logger.Info().Msg("Contract applied!")
			return output.EncodeOutput(flagOutputFormat, res, contractItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "contract name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	cmd.Flags().StringVarP(&contractPath, "contract", "f", "", "path or URL to the contract schema")
	cmd.Flags().StringVar(&description, "description", "", "description of the contract")
	cmd.Flags().StringVar(&projectName, "project", "", "project name used to scope the contract, if not set the contract will be created in the organization")

	return cmd
}
