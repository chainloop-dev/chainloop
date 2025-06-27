//
// Copyright 2025 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newGroupDeleteCmd() *cobra.Command {
	var groupName string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Ask for confirmation, unless the --yes flag is set
			if !flagYes {
				logger.Warn().Msgf("Are you sure you want to delete the group '%s'?", groupName)

				if err := confirmDeletion(); err != nil {
					return err
				}
			}

			if err := action.NewGroupDelete(actionOpts).Run(ctx, groupName); err != nil {
				return fmt.Errorf("removing group: %w", err)
			}

			logger.Info().Msgf("Group '%s' has been removed", groupName)
			return nil
		},
	}

	cmd.Flags().StringVar(&groupName, "name", "", "Name of the group to remove")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	return cmd
}
