//
// Copyright 2024 The Chainloop Authors.
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newWorkflowCreateCmd() *cobra.Command {
	var workflowName, description, project, team, contractRef string
	var public, skipIfExists bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new workflow",
		Example: `  chainloop workflow create --name [workflowName] --project [projectName]

  # Indicate an optional team name
  chainloop workflow create --name release --project skynet --team core-cyberdyne

  # Associate an existing contract referenced by its ID
  chainloop workflow create --name release --project skynet --contract deadbeed

  # Or create a new contract by pointing to a local file or URL
  chainloop workflow create --name release --project skynet --contract ./skynet.contract.yaml
  chainloop workflow create --name release --project skynet --contract https://skynet.org/contract.yaml
  `,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If contract flag is provided we want to either
			// 1 - make sure it exists and attach it
			// 2 - Create a new contract from a file or URL
			if contractRef != "" {
				// Try to find it by name
				c, err := action.NewWorkflowContractDescribe(actionOpts).Run(contractRef, 0)
				if err != nil || c == nil {
					createResp, err := action.NewWorkflowContractCreate(actionOpts).Run(fmt.Sprintf("%s-%s", project, workflowName), nil, contractRef)
					if err != nil {
						return err
					}
					contractRef = createResp.Name
				}
			}

			opts := &action.NewWorkflowCreateOpts{
				Name: workflowName, Team: team, Project: project, ContractName: contractRef, Description: description,
				Public: public,
			}

			wf, err := action.NewWorkflowCreate(actionOpts).Run(opts)
			if err != nil {
				if s, ok := status.FromError(err); ok {
					if s.Code() == codes.AlreadyExists {
						if skipIfExists {
							logger.Info().Msg("Workflow already exists")
							return nil
						}
					}
				}

				return err
			}

			// Print the workflow table
			if err := encodeOutput(wf, workflowItemTableOutput); err != nil {
				return fmt.Errorf("failed to print workflow: %w", err)
			}

			logger.Info().Msg("To Attest this workflow you'll need to provide an API token. See \"chainloop organization api-token\" command for more information.\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&workflowName, "name", "", "workflow name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&description, "description", "", "workflow description")

	cmd.Flags().StringVar(&project, "project", "", "project name")
	err = cmd.MarkFlagRequired("project")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&team, "team", "", "team name")
	cmd.Flags().StringVar(&contractRef, "contract", "", "the name of an existing contract or the path/URL to a contract file. If not provided an empty one will be created.")
	cmd.Flags().BoolVar(&public, "public", false, "is the workflow public")
	cmd.Flags().BoolVarP(&skipIfExists, "skip-if-exists", "f", false, "do not fail if the workflow with the provided name already exists")
	cmd.Flags().SortFlags = false

	return cmd
}
