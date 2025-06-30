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

func newGroupCreateCmd() *cobra.Command {
	var name, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new group",
		Example: `  chainloop group create --name [groupName]

  # With a description
  chainloop group create --name developers --description "Group for developers"
  `,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resp, err := action.NewGroupCreate(actionOpts).Run(cmd.Context(), name, description)
			if err != nil {
				return err
			}

			// Print the group details
			if err := encodeOutput(resp, groupItemTableOutput); err != nil {
				return fmt.Errorf("failed to print group: %w", err)
			}

			logger.Info().Msg("Group created successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "group name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&description, "description", "", "group description")
	cmd.Flags().SortFlags = false

	return cmd
}

// Format function for group output
func groupItemTableOutput(data *action.GroupCreateItem) error {
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
