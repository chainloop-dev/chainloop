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
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

// Get the membership entry associated to the current user for the given organization
func membershipFromOrg(ctx context.Context, name string) (*action.MembershipItem, error) {
	memberships, err := action.NewMembershipList(actionOpts).ListOrgs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing memberships: %w", err)
	}

	for _, m := range memberships {
		if m.Org.Name == name {
			return m, nil
		}
	}

	return nil, fmt.Errorf("organization %s not found", name)
}

func newOrganizationLeaveCmd() *cobra.Command {
	var orgName string
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "leave an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// To find the membership ID, we need to iterate and filter by org
			membership, err := membershipFromOrg(ctx, orgName)
			if err != nil {
				return fmt.Errorf("getting membership: %w", err)
			} else if membership == nil {
				return fmt.Errorf("organization %s not found", orgName)
			}

			fmt.Printf("You are about to leave the organization %q\n", membership.Org.Name)

			// Ask for confirmation
			if err := confirmDeletion(); err != nil {
				return err
			}

			// Membership deletion
			if err := action.NewMembershipLeave(actionOpts).Run(ctx, membership.ID); err != nil {
				return fmt.Errorf("deleting membership: %w", err)
			}

			logger.Info().Msg("Membership deleted")
			return nil
		},
	}

	cmd.Flags().StringVar(&orgName, "name", "", "organization name")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))
	return cmd
}
