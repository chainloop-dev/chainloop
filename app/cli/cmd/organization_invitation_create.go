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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newOrganizationInvitationCreateCmd() *cobra.Command {
	var receiverEmail, organizationID string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Invite a User to an Organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewOrgInvitationCreate(actionOpts).Run(
				context.Background(), organizationID, receiverEmail)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.OrgInvitationItem{res}, orgInvitationTableOutput)
		},
	}

	cmd.Flags().StringVar(&receiverEmail, "receiver", "", "Email of the user to invite")
	err := cmd.MarkFlagRequired("receiver")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&organizationID, "organization", "", "ID of the organization to invite the user to")
	err = cmd.MarkFlagRequired("organization")
	cobra.CheckErr(err)

	return cmd
}
