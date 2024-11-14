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

func newWorkflowListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List existing Workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, WorkflowListTableOutput)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show the full report")

	return cmd
}

func workflowItemTableOutput(workflow *action.WorkflowItem) error {
	return WorkflowListTableOutput([]*action.WorkflowItem{workflow})
}

func WorkflowListTableOutput(workflows []*action.WorkflowItem) error {
	if len(workflows) == 0 {
		fmt.Println("there are no workflows yet")
		return nil
	}

	headerRow := table.Row{"Name", "Project", "Contract", "Public", "Runner", "Last Run status", "Created At"}
	headerRowFull := table.Row{"Name", "Description", "Project", "Team", "Contract", "Public", "Runner", "Last Run status", "Created At"}

	t := newTableWriter()
	if full {
		t.AppendHeader(headerRowFull)
	} else {
		t.AppendHeader(headerRow)
	}

	for _, p := range workflows {
		var row table.Row
		var lastRunRunner, lastRunState string
		if lr := p.LastRun; lr != nil {
			lastRunRunner = lr.RunnerType
			lastRunState = lr.State
		}

		if !full {
			row = table.Row{
				p.Name, p.Project, p.ContractName, p.Public,
				lastRunRunner, lastRunState,
				p.CreatedAt.Format(time.RFC822),
			}
		} else {
			row = table.Row{
				p.Name, p.Description, p.Project, p.Team,
				p.ContractName, p.Public,
				lastRunRunner, lastRunState,
				p.CreatedAt.Format(time.RFC822),
			}
		}

		t.AppendRow(row)
	}
	t.Render()

	return nil
}
