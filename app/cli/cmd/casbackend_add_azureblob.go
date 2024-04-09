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
	"github.com/chainloop-dev/chainloop/internal/blobmanager/azureblob"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendAddAzureBlobStorageCmd() *cobra.Command {
	var storageAccountName, tenantID, clientID, clientSecret, container string
	cmd := &cobra.Command{
		Use:   "azure-blob",
		Short: "Register a Azure Blob Storage CAS Backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If we are setting the default, we list existing CAS backends
			// and ask the user to confirm the rewrite
			isDefault, err := cmd.Flags().GetBool("default")
			cobra.CheckErr(err)

			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts, ""); err != nil {
					return err
				} else if !confirmed {
					log.Info("Aborting...")
					return nil
				}
			}

			opts := &action.NewCASBackendAddOpts{
				Name:        name,
				Location:    fmt.Sprintf("%s/%s", storageAccountName, container),
				Provider:    azureblob.ProviderID,
				Description: description,
				Credentials: map[string]any{
					"tenantID":     tenantID,
					"clientID":     clientID,
					"clientSecret": clientSecret,
				},
				Default: isDefault,
			}

			res, err := action.NewCASBackendAdd(actionOpts).Run(opts)
			if err != nil {
				return err
			} else if res == nil {
				return nil
			}

			return encodeOutput([]*action.CASBackendItem{res}, casBackendListTableOutput)
		},
	}

	cmd.Flags().StringVar(&storageAccountName, "storage-account", "", "Storage Account Name")
	err := cmd.MarkFlagRequired("storage-account")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&tenantID, "tenant", "", "Active Directory Tenant ID")
	err = cmd.MarkFlagRequired("tenant")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&clientID, "client-id", "", "Service Principal Client ID")
	err = cmd.MarkFlagRequired("client-id")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Service Principal Client Secret")
	err = cmd.MarkFlagRequired("client-secret")
	cobra.CheckErr(err)

	cmd.Flags().StringVar(&container, "container", "chainloop", "Storage Container Name")
	return cmd
}
