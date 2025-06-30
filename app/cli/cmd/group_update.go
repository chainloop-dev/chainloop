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
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/spf13/cobra"
)

func newGroupUpdateCmd() *cobra.Command {
	var newName, newDescription string

	cmd := &cobra.Command{
		Use:   "update [groupName]",
		Short: "Update an existing group",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update a group name
  chainloop group update my-group --name new-name

  # Update a group description
  chainloop group update my-group --description "New description"

  # Update both name and description
  chainloop group update my-group --name new-name --description "New description"
  `,
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]

			// Check if at least one field is being updated
			if newName == "" && newDescription == "" {
				return fmt.Errorf("at least one of --name or --description must be provided")
			}

			// Prepare the arguments
			var namePtr, descPtr *string
			if newName != "" {
				namePtr = &newName
			}
			if newDescription != "" {
				descPtr = &newDescription
			}

			resp, err := action.NewGroupUpdate(actionOpts).Run(cmd.Context(), groupName, namePtr, descPtr)
			if err != nil {
				return err
			}

			// Print the updated group details
			if err := encodeOutput(resp, groupUpdateTableOutput); err != nil {
				return fmt.Errorf("failed to print group: %w", err)
			}

			logger.Info().Msg("Group updated successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&newName, "name", "", "new group name")
	cmd.Flags().StringVar(&newDescription, "description", "", "new group description")
	cmd.Flags().SortFlags = false

	return cmd
}

// Format function for group update output
func groupUpdateTableOutput(data *action.GroupUpdateItem) error {
	t := newTableWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Description", "Created At", "Updated At"})
	t.AppendRow(table.Row{
		data.ID,
		data.Name,
		data.Description,
		data.CreatedAt,
		data.UpdatedAt,
	})

	t.Render()

	return nil
}
