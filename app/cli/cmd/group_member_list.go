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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newGroupMemberListCmd() *cobra.Command {
	var paginationOpts = &options.OffsetPaginationOpts{}
	var groupName, memberEmail string
	var role string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"list", "ls"},
		Short:   "List members of a group",
		Example: `  # List all members of a group
  chainloop group members list --name developers

  # List only maintainers of a group
  chainloop group members list --name developers --role maintainer

  # List only members of a group
  chainloop group members list --name developers --role member

  # Filter by member email
  chainloop group members list --name developers --member-email user@example.com

  # Specify the page and page size
  chainloop group members list --name developers --page 2 --limit 10

  # Output in json format for scripts
  chainloop group members list --name developers --output json
`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if paginationOpts.Page < 1 {
				return fmt.Errorf("--page must be greater or equal than 1")
			}
			if paginationOpts.Limit < 1 {
				return fmt.Errorf("--limit must be greater or equal than 1")
			}

			// Validate role flag if provided
			if role != "" && role != "maintainer" && role != "member" {
				return fmt.Errorf("--role must be either 'maintainer' or 'member'")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			filterOpts := &action.GroupMemberListFilterOpts{
				GroupName:   groupName,
				MemberEmail: memberEmail,
				Role:        role,
			}

			res, err := action.NewGroupMemberList(actionOpts).Run(cmd.Context(), paginationOpts.Page, paginationOpts.Limit, filterOpts)
			if err != nil {
				return err
			}

			if err := encodeOutput(res, GroupMemberListTableOutput); err != nil {
				return err
			}

			pgResponse := res.Pagination

			if pgResponse.TotalPages >= paginationOpts.Page {
				inPage := min(paginationOpts.Limit, len(res.Members))
				lowerBound := (paginationOpts.Page - 1) * paginationOpts.Limit
				logger.Info().Msg(fmt.Sprintf("Showing [%d-%d] out of %d", lowerBound+1, lowerBound+inPage, pgResponse.TotalCount))
			}

			if pgResponse.TotalCount > pgResponse.Page*pgResponse.PageSize {
				logger.Info().Msg(fmt.Sprintf("Next page available: %d", pgResponse.Page+1))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&groupName, "name", "", "name of the group")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&memberEmail, "member-email", "", "filter by member email")
	cmd.Flags().StringVar(&role, "role", "", "filter by role (maintainer, member), by default all members are listed")
	paginationOpts.AddFlags(cmd)

	return cmd
}

func GroupMemberListTableOutput(memberListResult *action.GroupMemberListResult) error {
	if len(memberListResult.Members) == 0 {
		fmt.Println("there are no members in this group")
		return nil
	}

	headerRow := table.Row{"Email", "Role", "Added At"}

	t := newTableWriter()
	t.AppendHeader(headerRow)

	for _, m := range memberListResult.Members {
		row := table.Row{
			m.User.PrintUserProfileWithEmail(),
			m.Role,
			formatTime(m.AddedAt),
		}
		t.AppendRow(row)
	}
	t.Render()

	return nil
}
