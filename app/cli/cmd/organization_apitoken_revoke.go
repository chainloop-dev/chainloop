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

func newAPITokenRevokeCmd() *cobra.Command {
	var apiTokenID string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "revoke API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := action.NewAPITokenRevoke(actionOpts).Run(context.Background(), apiTokenID); err != nil {
				return fmt.Errorf("revoking API token: %w", err)
			}

			logger.Info().Msg("API token revoked!")
			return nil
		},
	}

	cmd.Flags().StringVar(&apiTokenID, "id", "", "API token ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	return cmd
}
