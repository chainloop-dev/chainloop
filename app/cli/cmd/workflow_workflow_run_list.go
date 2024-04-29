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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newWorkflowWorkflowRunListCmd() *cobra.Command {
	var paginationOpts = &options.PaginationOpts{
		DefaultLimit: 20,
	}

	var workflowID string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workflow runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRunList(actionOpts).Run(
				&action.WorkflowRunListOpts{
					WorkflowID: workflowID,
					Pagination: &action.PaginationOpts{
						Limit:      paginationOpts.Limit,
						NextCursor: paginationOpts.NextCursor,
					},
				},
			)
			if err != nil {
				return err
			}

			if err := encodeOutput(res.Result, workflowRunListTableOutput); err != nil {
				return err
			}

			next := res.PaginationMeta.NextCursor
			if next == "" {
				return nil
			}

			logger.Info().Msg("Pagination options \n\n")

			if next != "" {
				logger.Info().Msgf("--next %s\n", next)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workflowID, "workflow", "", "workflow ID")
	cmd.Flags().BoolVar(&full, "full", false, "full report")
	// Add pagination flags
	paginationOpts.AddFlags(cmd)

	return cmd
}

func workflowRunListTableOutput(runs []*action.WorkflowRunItem) error {
	if len(runs) == 0 {
		fmt.Println("there are no workflow runs yet")
		return nil
	}

	header := table.Row{"ID", "Workflow", "State", "Created At", "Runner"}
	if full {
		header = append(header, "Finished At", "Failure reason")
	}

	t := newTableWriter()
	t.AppendHeader(header)

	for _, p := range runs {
		wf := p.Workflow
		r := table.Row{p.ID, wf.NamespacedName(), p.State, p.CreatedAt.Format(time.RFC822), p.RunnerType}

		if full {
			var finishedAt string
			if p.FinishedAt != nil {
				finishedAt = p.FinishedAt.Format(time.RFC822)
			}
			r = append(r, finishedAt, p.Reason)
		}
		t.AppendRow(r)
	}
	t.Render()

	return nil
}
