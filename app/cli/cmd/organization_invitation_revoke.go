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

func newOrganizationInvitationRevokeCmd() *cobra.Command {
	var invitationID string
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a pending invitation",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := action.NewOrgInvitationRevoke(actionOpts).Run(context.Background(), invitationID); err != nil {
				return err
			}

			logger.Info().Msg("Invitation Revoked!")
			return nil
		},
	}

	cmd.Flags().StringVar(&invitationID, "id", "", "Invitation ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	return cmd
}
