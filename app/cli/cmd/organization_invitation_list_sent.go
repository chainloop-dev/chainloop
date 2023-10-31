//
// Copyright 2023 The Chainloop Authors.
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
	"context"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newOrganizationInvitationListSentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sent invitations",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewOrgInvitationListSent(actionOpts).Run(context.Background())
			if err != nil {
				return err
			}

			return encodeOutput(res, orgInvitationTableOutput)
		},
	}

	return cmd
}

func orgInvitationTableOutput(items []*action.OrgInvitationItem) error {
	if len(items) == 0 {
		fmt.Println("there are no sent invitations")
		return nil
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Org Name", "Receiver Email", "Status", "Created At"})

	for _, i := range items {
		t.AppendRow(table.Row{i.ID, i.Organization.Name, i.ReceiverEmail, i.Status, i.CreatedAt.Format(time.RFC822)})
		t.AppendSeparator()
	}

	t.Render()
	return nil
}
