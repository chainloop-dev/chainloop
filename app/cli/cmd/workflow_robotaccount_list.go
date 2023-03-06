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
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newWorkflowRobotAccountListCmd() *cobra.Command {
	var workflowID string
	var includeRevoked bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List robot accounts associated with a workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRobotAccountList(actionOpts).Run(workflowID, includeRevoked)
			if err != nil {
				return err
			}

			return encodeOutput(res, robotAccountListTableOutput)
		},
	}

	cmd.Flags().StringVar(&workflowID, "workflow", "", "workflow ID")
	cmd.Flags().BoolVarP(&includeRevoked, "all", "a", false, "show all robot accounts including revoked ones")
	err := cmd.MarkFlagRequired("workflow")
	cobra.CheckErr(err)

	return cmd
}

func robotAccountListTableOutput(robotAccounts []*action.WorkflowRobotAccountItem) error {
	if len(robotAccounts) == 0 {
		fmt.Println("there are no robot accounts yet")
		return nil
	}

	t := newTableWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Workflow ID", "Created At", "Revoked At"})
	for _, p := range robotAccounts {
		r := table.Row{p.ID, p.Name, p.WorkflowID, p.CreatedAt.Format(time.RFC822)}
		if p.RevokedAt != nil {
			r = append(r, p.RevokedAt.Format(time.RFC822))
		}

		t.AppendRow(r)
	}
	t.Render()

	if len(robotAccounts) == 1 && robotAccounts[0].Key != "" {
		// Output the token too
		fmt.Printf("\nSave the following token since it will not printed again: \n\n %s\n\n", robotAccounts[0].Key)
	}
	return nil
}
