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

func newCASBackendUpdateAzureBlobCmd() *cobra.Command {
	var backendName, tenantID, clientID, clientSecret string
	cmd := &cobra.Command{
		Use:   "azure-blob",
		Short: "Update a AzureBlob CAS Backend description, credentials or default status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If we are setting the default, we list existing CAS backends
			// and ask the user to confirm the rewrite
			isDefault, err := cmd.Flags().GetBool("default")
			cobra.CheckErr(err)

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			// If we are overriding the default we ask for confirmation
			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts, backendName); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			} else {
				// If we are removing the default we ask for confirmation too
				if confirmed, err := confirmDefaultCASBackendUnset(backendName, "You are setting the default CAS backend to false", actionOpts); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			}

			opts := &action.NewCASBackendUpdateOpts{
				Name:        backendName,
				Description: description,
				Credentials: map[string]any{
					"tenantID":     tenantID,
					"clientID":     clientID,
					"clientSecret": clientSecret,
				},
				Default: isDefault,
			}

			// this means that we are not updating credentials
			if tenantID == "" && clientID == "" && clientSecret == "" {
				opts.Credentials = nil
			}

			res, err := action.NewCASBackendUpdate(actionOpts).Run(opts)
			if err != nil {
				return err
			} else if res == nil {
				return nil
			}

			return encodeOutput(res, casBackendItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&backendName, "name", "", "CAS Backend name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&tenantID, "tenant", "", "Active Directory Tenant ID")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Service Principal Client ID")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Service Principal Client Secret")

	return cmd
}
