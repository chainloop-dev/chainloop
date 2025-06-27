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
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newGroupListCmd() *cobra.Command {
	var paginationOpts = &options.OffsetPaginationOpts{}
	var groupName, description, memberEmail string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List existing Groups",
		Example: `  # Let the default pagination apply
  chainloop group list

  # Specify the page and page size
  chainloop group list --page 2 --limit 10

  # Output in json format to paginate using scripts
  chainloop group list --page 2 --limit 10 --output json

  # Filter by group name
  chainloop group list --group-name developers

  # Filter by description
  chainloop group list --description "team members"

  # Filter by member email
  chainloop group list --member-email user@example.com

  # Show the full report
  chainloop group list --full
`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if paginationOpts.Page < 1 {
				return fmt.Errorf("--page must be greater or equal than 1")
			}
			if paginationOpts.Limit < 1 {
				return fmt.Errorf("--limit must be greater or equal than 1")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			filterOpts := &action.GroupListFilterOpts{
				GroupName:   groupName,
				Description: description,
				MemberEmail: memberEmail,
			}

			res, err := action.NewGroupList(actionOpts).Run(cmd.Context(), paginationOpts.Page, paginationOpts.Limit, filterOpts)
			if err != nil {
				return err
			}

			if err := encodeOutput(res, GroupListTableOutput); err != nil {
				return err
			}

			pgResponse := res.Pagination

			if pgResponse.TotalPages >= paginationOpts.Page {
				inPage := min(paginationOpts.Limit, len(res.Groups))
				lowerBound := (paginationOpts.Page - 1) * paginationOpts.Limit
				logger.Info().Msg(fmt.Sprintf("Showing [%d-%d] out of %d", lowerBound+1, lowerBound+inPage, pgResponse.TotalCount))
			}

			if pgResponse.TotalCount > pgResponse.Page*pgResponse.PageSize {
				logger.Info().Msg(fmt.Sprintf("Next page available: %d", pgResponse.Page+1))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show the full report")
	cmd.Flags().StringVar(&groupName, "group-name", "", "filter by group name")
	cmd.Flags().StringVar(&description, "description", "", "filter by description")
	cmd.Flags().StringVar(&memberEmail, "member-email", "", "filter by member email")
	paginationOpts.AddFlags(cmd)

	return cmd
}

func GroupListTableOutput(groupListResult *action.GroupListResult) error {
	if len(groupListResult.Groups) == 0 {
		fmt.Println("there are no groups yet")
		return nil
	}

	headerRow := table.Row{"ID", "Name", "Description", "Created At"}
	headerRowFull := table.Row{"ID", "Name", "Description", "Created At", "Updated At"}

	t := newTableWriter()
	if full {
		t.AppendHeader(headerRowFull)
	} else {
		t.AppendHeader(headerRow)
	}

	for _, g := range groupListResult.Groups {
		var row table.Row

		if !full {
			row = table.Row{
				g.ID, g.Name, g.Description,
				formatTime(g.CreatedAt),
			}
		} else {
			row = table.Row{
				g.ID, g.Name, g.Description,
				formatTime(g.CreatedAt), formatTime(g.UpdatedAt),
			}
		}

		t.AppendRow(row)
	}
	t.Render()

	return nil
}

// Helper function to format time string
func formatTime(timeStr string) string {
	if timeStr == "" {
		return ""
	}

	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr
	}

	return t.Format(time.RFC822)
}
