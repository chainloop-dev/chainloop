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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newWorkflowIntegrationAttachCmd() *cobra.Command {
	var options []string
	var integrationID, workflowID string

	cmd := &cobra.Command{
		Use:     "attach",
		Short:   "Attach an existing registered integration to a workflow",
		Example: `  chainloop workflow integration attach --workflow deadbeef --integration beefdoingwell --options projectName=MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := parseKeyValOpts(options)
			if err != nil {
				return err
			}

			res, err := action.NewWorkflowIntegrationAttach(actionOpts).Run(integrationID, workflowID, opts)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.IntegrationAttachmentItem{res}, integrationAttachmentListTableOutput)
		},
	}

	cmd.Flags().StringVar(&integrationID, "integration", "", "ID of the integration already registered in this organization")
	cobra.CheckErr(cmd.MarkFlagRequired("integration"))

	cmd.Flags().StringVar(&workflowID, "workflow", "", "ID of the workflow to attach this integration")
	cobra.CheckErr(cmd.MarkFlagRequired("workflow"))

	cmd.Flags().StringSliceVar(&options, "options", nil, "integration attachment arguments")
	cmd.AddCommand(newWorkflowIntegrationAttachDependencyTrackCmd())

	return cmd
}
