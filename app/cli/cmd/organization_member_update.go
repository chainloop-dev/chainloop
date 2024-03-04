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
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newOrganizationMemberUpdateCmd() *cobra.Command {
	var membershipID, role string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a member of the organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			var found bool
			for _, r := range action.AvailableRoles {
				if string(r) == role {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("role %q is not available. Available roles are %s", role, action.AvailableRoles)
			}

			res, err := action.NewMembershipUpdate(actionOpts).ChangeRole(cmd.Context(), membershipID, role)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.MembershipItem{res}, orgMembershipsTableOutput)
		},
	}

	cmd.Flags().StringVar(&membershipID, "id", "", "Membership ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&role, "role", string(action.RoleViewer), fmt.Sprintf("Role of the user in the organization, available %s", action.AvailableRoles))
	cobra.CheckErr(err)

	return cmd
}
