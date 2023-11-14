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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newWorkflowUpdateCmd() *cobra.Command {
	var workflowID, name, project, team string
	var public bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.NewWorkflowUpdateOpts{}
			if cmd.Flags().Changed("name") {
				opts.Name = &name
			}
			if cmd.Flags().Changed("team") {
				opts.Team = &team
			}
			if cmd.Flags().Changed("project") {
				opts.Project = &project
			}
			if cmd.Flags().Changed("public") {
				opts.Public = &public
			}

			res, err := action.NewWorkflowUpdate(actionOpts).Run(context.Background(), workflowID, opts)
			if err != nil {
				return err
			}

			logger.Info().Msg("Workflow updated!")
			return encodeOutput([]*action.WorkflowItem{res}, WorkflowListTableOutput)
		},
	}

	cmd.Flags().StringVar(&workflowID, "id", "", "workflow ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&name, "name", "", "workflow name")
	cmd.Flags().StringVar(&team, "team", "", "team name")
	cmd.Flags().StringVar(&project, "project", "", "project name")
	cmd.Flags().BoolVar(&public, "public", false, "is the workflow public")

	return cmd
}
