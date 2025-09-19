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
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newOrganizationMemberList() *cobra.Command {
	var (
		paginationOpts = &options.OffsetPaginationOpts{}
		name           string
		email          string
		role           string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the members of the current organization",
		Example: `  # Let the default pagination apply
  chainloop organization member list

  # Specify the page and page size
  chainloop organization member list --page 2 --limit 10

  # Filter by name
  chainloop organization member list --name alice

  # Filter by email
  chainloop organization member list --email alice@example.com

  # Filter by role
  chainloop organization member list --role admin

  # Combine filters and pagination
  chainloop organization member list --role admin --page 2 --limit 5
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
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.ListMembersOpts{}

			switch {
			case name != "":
				opts.Name = &name
			case email != "":
				opts.Email = &email
			case role != "":
				opts.Role = &role
			}

			res, err := action.NewMembershipList(actionOpts).ListMembers(cmd.Context(), paginationOpts.Page, paginationOpts.Limit, opts)
			if err != nil {
				return err
			}

			if err := output.EncodeOutput(flagOutputFormat, res, orgMembershipsTableOutput); err != nil {
				return err
			}

			pgResponse := res.PaginationMeta

			if pgResponse.TotalPages >= paginationOpts.Page {
				inPage := min(paginationOpts.Limit, len(res.Memberships))
				lowerBound := (paginationOpts.Page - 1) * paginationOpts.Limit
				logger.Info().Msg(fmt.Sprintf("Showing [%d-%d] out of %d", lowerBound+1, lowerBound+inPage, pgResponse.TotalCount))
			}

			if pgResponse.TotalCount > pgResponse.Page*pgResponse.PageSize {
				logger.Info().Msg(fmt.Sprintf("Next page available: %d", pgResponse.Page+1))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Filter by member name or last name")
	cmd.Flags().StringVar(&email, "email", "", "Filter by member email")
	cmd.Flags().StringVar(&role, "role", "", fmt.Sprintf("Role of the user in the organization, available %s", action.AvailableRoles[:3]))
	paginationOpts.AddFlags(cmd)

	return cmd
}

func orgMembershipsTableOutput(res *action.ListMembershipResult) error {
	t := output.NewTableWriter()
	t.AppendHeader(table.Row{"ID", "Email", "Role", "Joined At"})

	for _, m := range res.Memberships {
		t.AppendRow(table.Row{
			m.ID,
			m.User.PrintUserProfileWithEmail(),
			m.Role,
			m.CreatedAt.Format(time.RFC822),
		})
		t.AppendSeparator()
	}

	t.Render()
	return nil
}
