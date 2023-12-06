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
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

// Get the membership entry associated to the current user for the given organization
func membershipFromOrg(orgID string) (*action.MembershipItem, error) {
	memberships, err := action.NewMembershipList(actionOpts).Run()
	if err != nil {
		return nil, fmt.Errorf("listing memberships: %w", err)
	}

	for _, m := range memberships {
		if m.Org.ID == orgID {
			return m, nil
		}
	}

	return nil, fmt.Errorf("organization %s not found", orgID)
}

func newOrganizationLeaveCmd() *cobra.Command {
	var orgID string
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "leave an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			// To find the membership ID, we need to iterate and filter by org
			membership, err := membershipFromOrg(orgID)
			if err != nil {
				return fmt.Errorf("getting membership: %w", err)
			} else if membership == nil {
				return fmt.Errorf("organization %s not found", orgID)
			}

			if membership.Current {
				return fmt.Errorf("organization with ID %s is marked as 'current'", orgID)
			}

			fmt.Printf("You are about to leave the organization %q\n", membership.Org.Name)

			// Ask for confirmation
			if err := confirmDeletion(); err != nil {
				return err
			}

			// Membership deletion
			if err := action.NewMembershipDelete(actionOpts).Run(context.Background(), membership.ID); err != nil {
				return fmt.Errorf("deleting membership: %w", err)
			}

			logger.Info().Msg("Membership deleted")
			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "id", "", "organization ID to leave")
	cobra.CheckErr(cmd.MarkFlagRequired("id"))
	return cmd
}
