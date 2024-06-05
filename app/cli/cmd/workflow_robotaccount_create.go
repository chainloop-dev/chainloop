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

func newWorkflowRobotAccountCreateCmd() *cobra.Command {
	var workflowID, accName string

	cmd := &cobra.Command{
		Use:        "create",
		Short:      "Create a Robot Account associated with a workflow",
		Deprecated: "Please use 'chainloop org api-token' instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRobotAccountCreate(actionOpts).Run(workflowID, accName)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.WorkflowRobotAccountItem{res}, robotAccountListTableOutput)
		},
	}

	cmd.Flags().StringVar(&workflowID, "workflow", "", "workflow ID")
	cmd.Flags().StringVar(&accName, "name", "", "key name")

	err := cmd.MarkFlagRequired("workflow")
	cobra.CheckErr(err)

	return cmd
}
