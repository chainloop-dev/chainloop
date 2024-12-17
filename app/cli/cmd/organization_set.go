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
	"github.com/spf13/viper"
)

func newOrganizationSet() *cobra.Command {
	var orgName string
	var setDefault bool

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the current organization to be used by this CLI",
		Example: `
  # Set the current organization to be used by this CLI
  $ chainloop org set --name my-org

  # Optionally set the organization as the default one for all clients by storing the preference server-side
  $ chainloop org set --name my-org --default
		`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			// To find the membership ID, we need to iterate and filter by org
			membership, err := membershipFromOrg(ctx, orgName)
			if err != nil {
				return fmt.Errorf("getting membership: %w", err)
			} else if membership == nil {
				return fmt.Errorf("organization %s not found", orgName)
			}

			// Set local state
			viper.Set(confOptions.organization.viperKey, orgName)
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("writing config file: %w", err)
			}

			// change the state server side
			if setDefault {
				var err error
				membership, err = action.NewMembershipSet(actionOpts).Run(ctx, membership.ID)
				if err != nil {
					return err
				}
			}

			logger.Info().Msg("Organization switched!")
			return encodeOutput([]*action.MembershipItem{membership}, orgMembershipTableOutput)
		},
	}

	cmd.Flags().StringVar(&orgName, "name", "", "organization name to make the switch")
	cmd.Flags().BoolVar(&setDefault, "default", false, "set this organization as the default one for all clients")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))

	return cmd
}
