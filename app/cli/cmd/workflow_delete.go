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

func newWorkflowDeleteCmd() *cobra.Command {
	var name, projectName string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an existing workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := action.NewWorkflowDelete(actionOpts).Run(name, projectName)
			if err == nil {
				logger.Info().Msg("Workflow deleted!")
			}
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "workflow name")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))

	cmd.Flags().StringVar(&projectName, "project", "", "project name")
	cobra.CheckErr(cmd.MarkFlagRequired("project"))

	return cmd
}
