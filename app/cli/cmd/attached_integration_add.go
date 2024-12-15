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

func newAttachedIntegrationAttachCmd() *cobra.Command {
	var options []string
	var integrationName, workflowName, projectName string

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"attach"},
		Short:   "Attach an existing registered integration to a workflow",
		Example: `  chainloop integration attached add --workflow deadbeef --project my-project --integration beefdoingwell --opt projectName=MyProject --opt projectVersion=1.0.0`,
		RunE: func(_ *cobra.Command, args []string) error {
			// Find the integration to extract the kind of integration we care about
			integration, err := action.NewRegisteredIntegrationDescribe(actionOpts).Run(integrationName)
			if err != nil {
				return err
			}

			// Retrieve schema for validation and options marshaling
			item, err := action.NewAvailableIntegrationDescribe(actionOpts).Run(integration.Kind)
			if err != nil {
				return err
			}

			// Parse and validate options
			opts, err := parseAndValidateOpts(options, item.Attachment)
			if err != nil {
				// Show schema table if validation fails
				if err := renderSchemaTable("Available options", item.Attachment.Properties); err != nil {
					return err
				}
				return err
			}

			res, err := action.NewAttachedIntegrationAdd(actionOpts).Run(integrationName, workflowName, projectName, opts)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.AttachedIntegrationItem{res}, attachedIntegrationListTableOutput)
		},
	}

	cmd.Flags().StringVar(&integrationName, "integration", "", "Name of the integration already registered in this organization")
	cobra.CheckErr(cmd.MarkFlagRequired("integration"))

	cmd.Flags().StringVar(&workflowName, "workflow", "", "name of the workflow to attach this integration")
	cobra.CheckErr(cmd.MarkFlagRequired("workflow"))

	cmd.Flags().StringVar(&projectName, "project", "", "name of the project the workflow belongs to")
	cobra.CheckErr(cmd.MarkFlagRequired("project"))

	// StringSlice seems to struggle with comma-separated values such as p12 jsonKeys provided as passwords
	// So we need to use StringArrayVar instead
	cmd.Flags().StringArrayVar(&options, "opt", nil, "integration attachment arguments")

	return cmd
}
