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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendDeleteCmd() *cobra.Command {
	var backendID string

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a CAS Backend from your organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			if confirmed, err := confirmDefaultCASBackendRemoval(actionOpts, backendID); err != nil {
				return err
			} else if !confirmed {
				log.Info("Aborting...")
				return nil
			}

			if err := action.NewCASBackendDelete(actionOpts).Run(backendID); err != nil {
				return err
			}

			logger.Info().Msg("Backend deleted")

			return nil
		},
	}

	cmd.Flags().StringVar(&backendID, "id", "", "CAS Backend ID")
	cobra.CheckErr(cmd.MarkFlagRequired("id"))
	return cmd
}
