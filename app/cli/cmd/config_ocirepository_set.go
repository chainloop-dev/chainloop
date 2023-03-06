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

func newOCIRepositoryCreateCmd() *cobra.Command {
	var repo, username, password string

	cmd := &cobra.Command{
		Use:   "set-oci-repo",
		Short: "Set the OCI repository associated with your current org",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.NewOCIRepositorySaveOpts{
				Repo: repo, Username: username, Password: password,
			}

			err := action.NewOCIRepositorySave(actionOpts).Run(opts)
			if err != nil {
				return err
			}

			logger.Info().Msg("repository saved")
			return nil
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
