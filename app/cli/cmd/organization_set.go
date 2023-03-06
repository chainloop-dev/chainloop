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
	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newOrganizationSet() *cobra.Command {
	var membershipID string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the current organization associated with this user",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewMembershipSet(actionOpts).Run(membershipID)
			if err != nil {
				return err
			}

			logger.Info().Msg("Organization switched!")
			return encodeOutput([]*action.MembershipItem{res}, orgMembershipTableOutput)
		},
	}

	cmd.Flags().StringVar(&membershipID, "id", "", "membership ID to make the switch")
	cobra.CheckErr(cmd.MarkFlagRequired("id"))

	return cmd
}
