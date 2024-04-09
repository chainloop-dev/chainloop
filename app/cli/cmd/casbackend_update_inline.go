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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendUpdateInlineCmd() *cobra.Command {
	var backendID string
	cmd := &cobra.Command{
		Use:   "inline",
		Short: "Update the Inline, fallback CAS Backend description or default status",
		RunE: func(cmd *cobra.Command, args []string) error {
			isDefault, err := cmd.Flags().GetBool("default")
			cobra.CheckErr(err)

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)

			// If we are overriding the default we ask for confirmation
			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts, backendID); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			} else {
				// If we are removing the default we ask for confirmation too
				if confirmed, err := confirmDefaultCASBackendUnset(backendID, "You are setting the default CAS backend to false", actionOpts); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			}

			opts := &action.NewCASBackendUpdateOpts{
				Name:        name,
				ID:          backendID,
				Description: description,
				Default:     isDefault,
			}

			res, err := action.NewCASBackendUpdate(actionOpts).Run(opts)
			if err != nil {
				return err
			} else if res == nil {
				return nil
			}

			return encodeOutput([]*action.CASBackendItem{res}, casBackendListTableOutput)
		},
	}

	cmd.Flags().StringVar(&backendID, "id", "", "CAS Backend ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)
	return cmd
}
