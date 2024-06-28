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
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newWorkflowContractListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List contracts",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowContractList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, contractListTableOutput)
		},
	}

	return cmd
}

func contractItemTableOutput(contract *action.WorkflowContractItem) error {
	return contractListTableOutput([]*action.WorkflowContractItem{contract})
}

func contractListTableOutput(contracts []*action.WorkflowContractItem) error {
	t := newTableWriter()

	t.AppendHeader(table.Row{"Name", "Latest Revision", "Created At", "# Workflows"})
	for _, p := range contracts {
		t.AppendRow(table.Row{p.Name, p.LatestRevision, p.CreatedAt.Format(time.RFC822), len(p.Workflows)})
	}

	t.Render()

	return nil
}
