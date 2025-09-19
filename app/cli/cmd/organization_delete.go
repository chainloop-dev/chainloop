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

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newOrganizationDeleteCmd() *cobra.Command {
	var orgName string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an organization",
		Long:  "Delete an organization. Only organization owners can delete an organization.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			fmt.Printf("You are about to delete the organization %q\n", orgName)
			fmt.Println("This action will permanently delete the organization and all its data.")

			// Ask for confirmation
			if err := confirmDeletion(); err != nil {
				return err
			}

			if err := action.NewOrganizationDelete(actionOpts).Run(ctx, orgName); err != nil {
				return fmt.Errorf("deleting organization: %w", err)
			}

			// Clear local state if we just deleted the current organization
			if err := setLocalOrganization(""); err != nil {
				return fmt.Errorf("writing config file: %w", err)
			}

			logger.Info().Str("organization", orgName).Msg("Organization deleted")
			return nil
		},
	}

	cmd.Flags().StringVar(&orgName, "name", "", "organization name")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))
	return cmd
}
