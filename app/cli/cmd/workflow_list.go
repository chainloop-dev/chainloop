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

const (
	// defaultPageSize is the default page size
	defaultPageSize = 15
	// defaultPage is the default page
	defaultPage = 1
)

var (
	// page is the current page number
	page int
	// pageSize is the number of workflows per page
	pageSize int
)

func newWorkflowListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List existing Workflows",
		Example: `  # Let the default pagination apply
  chainloop workflow list

  # Specify the page and page size
  chainloop workflow list --page 2 --page-size 10

  # Output in json format to paginate using scripts
  chainloop workflow list --page 2 --page-size 10 --output json

  # Show the full report
  chainloop workflow list --full
`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if page < 1 {
				return fmt.Errorf("--page must be greater or equal than 1")
			}
			if pageSize < 1 {
				return fmt.Errorf("--page-size must be greater or equal than 1")
			}

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			res, err := action.NewWorkflowList(actionOpts).Run(page, pageSize)
			if err != nil {
				return err
			}

			if err := encodeOutput(res, WorkflowListTableOutput); err != nil {
				return err
			}

			pgResponse := res.Pagination

			logger.Info().Msg(fmt.Sprintf("Showing %d out of %d", len(res.Workflows), pgResponse.TotalCount))

			if pgResponse.TotalCount > pgResponse.Page*pgResponse.PageSize {
				logger.Info().Msg(fmt.Sprintf("Next page available: %d", pgResponse.Page+1))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show the full report")
	cmd.Flags().IntVar(&page, "page", defaultPage, "page number")
	cmd.Flags().IntVar(&pageSize, "page-size", defaultPageSize, "number of workflows per page")

	return cmd
}

func workflowItemTableOutput(workflow *action.WorkflowItem) error {
	return WorkflowListTableOutput(&action.WorkflowListResult{Workflows: []*action.WorkflowItem{workflow}})
}

func WorkflowListTableOutput(workflowListResult *action.WorkflowListResult) error {
	if len(workflowListResult.Workflows) == 0 {
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

	for _, p := range workflowListResult.Workflows {
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
