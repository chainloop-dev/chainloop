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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newOrganizationList() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the organizations this user has access to",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewMembershipList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, orgMembershipTableOutput)
		},
	}

	return cmd
}

func orgMembershipTableOutput(items []*action.MembershipItem) error {
	if len(items) == 0 {
		fmt.Println("you have no access to any organization yet")
		return nil
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"Org ID", "Org Name", "Current", "Role", "Joined At"})

	for _, i := range items {
		t.AppendRow(table.Row{i.Org.ID, i.Org.Name, i.Current, i.Role, i.CreatedAt.Format(time.RFC822)})
		t.AppendSeparator()
	}

	t.Render()
	return nil
}
