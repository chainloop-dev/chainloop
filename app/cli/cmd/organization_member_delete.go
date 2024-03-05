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
func loadMembershipCurrentOrg(ctx context.Context, membershipID string) (*action.MembershipItem, error) {
	memberships, err := action.NewMembershipList(actionOpts).ListMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing memberships: %w", err)
	}

	for _, m := range memberships {
		if m.ID == membershipID {
			return m, nil
		}
	}

	return nil, fmt.Errorf("membership %s not found", membershipID)
}

func newOrganizationMemberDeleteCmd() *cobra.Command {
	var membershipID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove a member from the current organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			m, err := loadMembershipCurrentOrg(ctx, membershipID)
			if err != nil {
				return fmt.Errorf("getting membership: %w", err)
			}

			fmt.Printf("You are about to remove the user %q from the organization %q\n", m.User.Email, m.Org.Name)

			// Ask for confirmation
			if err := confirmDeletion(); err != nil {
				return err
			}

			if err := action.NewMembershipDelete(actionOpts).Run(ctx, membershipID); err != nil {
				return err
			}

			logger.Info().Msg("Member deleted")
			return nil
		},
	}

	cmd.Flags().StringVar(&membershipID, "id", "", "Membership ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	return cmd
}
