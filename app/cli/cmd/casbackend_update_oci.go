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
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendUpdateOCICmd() *cobra.Command {
	var backendID, username, password string
	cmd := &cobra.Command{
		Use:   "oci",
		Short: "Update a OCI CAS Backend description, credentials or default status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If we are setting the default, we list existing CAS backends
			// and ask the user to confirm the rewrite
			isDefault, err := cmd.Flags().GetBool("default")
			cobra.CheckErr(err)

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			// If we are overriding the default we ask for confirmation
			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts, backendID); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			} else { // If we are removing the default we ask for confirmation too
				if confirmed, err := confirmDefaultCASBackendRemoval(actionOpts, backendID); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			}

			opts := &action.NewCASBackendUpdateOpts{
				ID:          backendID,
				Description: description,
				Credentials: map[string]any{
					"username": username,
					"password": password,
				},
				Default: isDefault,
			}

			if username == "" && password == "" {
				opts.Credentials = nil
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

	cmd.Flags().StringVarP(&username, "username", "u", "", "registry username")

	cmd.Flags().StringVarP(&password, "password", "p", "", "registry password")
	return cmd
}

// If we are removing the default we confirm too
func confirmDefaultCASBackendRemoval(actionOpts *action.ActionsOpts, id string) (bool, error) {
	// get existing backends
	backends, err := action.NewCASBackendList(actionOpts).Run()
	if err != nil {
		return false, fmt.Errorf("failed to list existing CAS backends: %w", err)
	}

	for _, b := range backends {
		// We are removing ourselves as the default, ask the user to confirm
		if b.Default && b.ID == id {
			// Ask the user to confirm the override
			fmt.Println("This is the default CAS backend and your are setting default=false.\nPlease confirm to continue y/N: ")
			var gotChallenge string
			fmt.Scanln(&gotChallenge)

			// If the user does not confirm, we are done
			if gotChallenge != "y" && gotChallenge != "Y" {
				return false, nil
			}
		}
	}

	return true, nil
}
