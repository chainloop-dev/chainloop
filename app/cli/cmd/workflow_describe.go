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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newWorkflowDescribeCmd() *cobra.Command {
	var workflowName, projectName string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe an existing workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			wf, err := action.NewWorkflowDescribe(actionOpts).Run(cmd.Context(), workflowName, projectName)
			if err != nil {
				return err
			}

			return encodeOutput(wf, workflowItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&workflowName, "name", "", "workflow name")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))

	cmd.Flags().StringVar(&projectName, "project", "", "project name")
	cobra.CheckErr(cmd.MarkFlagRequired("project"))
	return cmd
}
