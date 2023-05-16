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
	"errors"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newWorkflowIntegrationAttachDependencyTrackCmd() *cobra.Command {
	var integrationID, workflowID, projectID, projectName string

	cmd := &cobra.Command{
		Use:   "dependency-track",
		Short: "Attach a Dependency-Track integration to this workflow",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" && projectName == "" {
				return errors.New("either a Dependency-Track --project-id or the --project-name flag must be set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := action.NewWorkflowIntegrationAttach(actionOpts).RunDependencyTrack(integrationID, workflowID, projectID, projectName)
			if err != nil {
				return err
			}

			logger.Info().Msg("Integration attached successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&integrationID, "integration", "", "ID of the integration already registered in this organization")
	cobra.CheckErr(cmd.MarkFlagRequired("integration"))

	cmd.Flags().StringVar(&workflowID, "workflow", "", "ID of the workflow to attach this integration")
	cobra.CheckErr(cmd.MarkFlagRequired("workflow"))

	cmd.Flags().StringVar(&projectID, "project-id", "", "Identifier of the Dependency-Track you want this integration to talk to")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Create a project with the provided name instead of using an existing one")
	cmd.MarkFlagsMutuallyExclusive("project-id", "project-name")

	return cmd
}
