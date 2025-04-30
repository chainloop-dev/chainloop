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
	"slices"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newWorkflowWorkflowRunListCmd() *cobra.Command {
	var paginationOpts = &options.PaginationOpts{
		DefaultLimit: 50,
	}

	var workflowName, projectName, status string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workflow runs",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if status != "" && !slices.Contains(listAvailableWorkflowStatusFlag(), status) {
				return fmt.Errorf("invalid status %q, please chose one of: %v", status, listAvailableWorkflowStatusFlag())
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRunList(actionOpts).Run(
				&action.WorkflowRunListOpts{
					WorkflowName: workflowName,
					ProjectName:  projectName,
					Pagination: &action.PaginationOpts{
						Limit:      paginationOpts.Limit,
						NextCursor: paginationOpts.NextCursor,
					},
					Status: status,
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

			logger.Info().Msg("Pagination options \n")

			if next != "" {
				logger.Info().Msgf("--next %s\n", next)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workflowName, "workflow", "", "workflow name")
	cmd.Flags().StringVar(&projectName, "project", "", "project name")
	cmd.Flags().BoolVar(&full, "full", false, "full report")
	cmd.Flags().StringVar(&status, "status", "", fmt.Sprintf("filter by workflow run status: %v", listAvailableWorkflowStatusFlag()))
	// Add pagination flags
	paginationOpts.AddFlags(cmd)

	return cmd
}

func workflowRunListTableOutput(runs []*action.WorkflowRunItem) error {
	if len(runs) == 0 {
		fmt.Println("there are no workflow runs yet")
		return nil
	}

	header := table.Row{"ID", "Workflow", "Version", "State", "Created At", "Runner"}
	if full {
		header = append(header, "Finished At", "Failure reason")
	}

	t := newTableWriter()
	t.AppendHeader(header)

	for _, p := range runs {
		wf := p.Workflow
		r := table.Row{p.ID, wf.NamespacedName(), versionString(p.ProjectVersion), p.State, p.CreatedAt.Format(time.RFC822), p.RunnerType}

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

func versionString(p *action.ProjectVersion) string {
	versionString := p.Version
	if versionString == "" {
		return ""
	}

	if !p.Prerelease {
		return versionString
	}

	return fmt.Sprintf("%s (prerelease)", p.Version)
}

// listAvailableWorkflowStatusFlag returns a list of available workflow status flags
func listAvailableWorkflowStatusFlag() []string {
	m := action.WorkflowRunStatus()
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}

	// Sort the list of status
	sort.Strings(r)

	return r
}
