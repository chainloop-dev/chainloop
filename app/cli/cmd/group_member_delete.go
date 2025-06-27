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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newGroupMemberDeleteCmd() *cobra.Command {
	var groupName, memberEmail string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove a member from a group",
		Example: `  # Remove a member from a group
  chainloop group member delete --name developers --email user@example.com
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Ask for confirmation, unless the --yes flag is set
			if !flagYes {
				logger.Warn().Msgf("You are about to remove the user %q from the group %q\n", memberEmail, groupName)

				if err := confirmDeletion(); err != nil {
					return err
				}
			}

			if err := action.NewGroupMemberDelete(actionOpts).Run(cmd.Context(), groupName, memberEmail); err != nil {
				return err
			}

			logger.Info().Msgf("Member %s successfully removed from group %s", memberEmail, groupName)
			return nil
		},
	}

	cmd.Flags().StringVar(&groupName, "name", "", "name of the group")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&memberEmail, "email", "", "email of the member to remove")
	err = cmd.MarkFlagRequired("email")
	cobra.CheckErr(err)

	return cmd
}
