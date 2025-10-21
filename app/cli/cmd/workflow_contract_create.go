//
// Copyright 2024-2025 The Chainloop Authors.
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

func newWorkflowContractCreateCmd() *cobra.Command {
	var name, description, contractPath, projectName string
	var contractName string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new contract",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate and extract the contract name
			var err error
			contractName, err = action.ValidateAndExtractName(name, contractPath)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var desc *string
			if cmd.Flags().Changed("description") {
				desc = &description
			}
			res, err := action.NewWorkflowContractCreate(ActionOpts).Run(contractName, desc, contractPath, projectName)
			if err != nil {
				return err
			}

			logger.Info().Msg("Contract created!")
			return output.EncodeOutput(flagOutputFormat, res, contractItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "contract name")
	cmd.Flags().StringVarP(&contractPath, "contract", "f", "", "path or URL to the contract schema")
	cmd.Flags().StringVar(&description, "description", "", "description of the contract")
	cmd.Flags().StringVar(&projectName, "project", "", "project name used to scope the contract, if not set the contract will be created in the organization")

	return cmd
}
