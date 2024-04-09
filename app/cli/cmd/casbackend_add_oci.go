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

func newCASBackendAddOCICmd() *cobra.Command {
	var repo, username, password string
	cmd := &cobra.Command{
		Use:   "oci",
		Short: "Register a OCI CAS Backend",
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
				Name:     name,
				Location: repo, Description: description,
				Provider: "OCI",
				Credentials: map[string]any{
					"username": username,
					"password": password,
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

	cmd.Flags().StringVar(&repo, "repo", "", "FQDN repository name, including path")
	err := cmd.MarkFlagRequired("repo")
	cobra.CheckErr(err)

	cmd.Flags().StringVarP(&username, "username", "u", "", "registry username")
	err = cmd.MarkFlagRequired("username")
	cobra.CheckErr(err)

	cmd.Flags().StringVarP(&password, "password", "p", "", "registry password")
	err = cmd.MarkFlagRequired("password")
	cobra.CheckErr(err)
	return cmd
}
