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

func newAttachedIntegrationDeleteCmd() *cobra.Command {
	var attachmentID string

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"detach"},
		Short:   "Detach an integration that's attached to a workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := action.NewAttachedIntegrationDelete(actionOpts).Run(attachmentID); err != nil {
				return err
			}

			logger.Info().Msg("integration detached!")
			return nil
		},
	}

	cmd.Flags().StringVar(&attachmentID, "id", "", "ID of the existing attachment")
	cobra.CheckErr(cmd.MarkFlagRequired("id"))

	return cmd
}
