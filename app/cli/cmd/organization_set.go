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
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newOrganizationSet() *cobra.Command {
	var orgID string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the current organization associated with this user",
		RunE: func(cmd *cobra.Command, args []string) error {
			// To change the current organization, we need to find the membership ID
			memberships, err := action.NewMembershipList(actionOpts).Run()
			if err != nil {
				return err
			}

			var membershipID string
			for _, m := range memberships {
				if m.Org.ID == orgID {
					membershipID = m.ID
					break
				}
			}

			if membershipID == "" {
				return fmt.Errorf("organization %s not found", orgID)
			}

			m, err := action.NewMembershipSet(actionOpts).Run(membershipID)
			if err != nil {
				return err
			}

			logger.Info().Msg("Organization switched!")
			return encodeOutput([]*action.MembershipItem{m}, orgMembershipTableOutput)
		},
	}

	cmd.Flags().StringVar(&orgID, "id", "", "organization ID to make the switch")
	cobra.CheckErr(cmd.MarkFlagRequired("id"))

	return cmd
}
