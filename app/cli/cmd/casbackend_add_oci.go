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

			description, err := cmd.Flags().GetString("description")
			cobra.CheckErr(err)

			if isDefault {
				if confirmed, err := confirmDefaultCASBackendOverride(actionOpts); err != nil {
					return err
				} else if !confirmed {
					fmt.Println("Aborting...")
				}
			}

			name := repo
			// The backend description contains the repository information and the optionally provided description
			if description != "" {
				name = fmt.Sprintf("%s\n%s", name, description)
			}

			opts := &action.NewCASBackendOCIAddOpts{
				Repo: repo, Username: username, Password: password, Default: isDefault, Name: name,
			}

			res, err := action.NewCASBackendAddOCI(actionOpts).Run(opts)
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

// confirmDefaultCASBackendOverride asks the user to confirm the override of the default CAS backend
// in the event that there is one already set.
func confirmDefaultCASBackendOverride(actionOpts *action.ActionsOpts) (bool, error) {
	// get existing backends
	backends, err := action.NewCASBackendList(actionOpts).Run()
	if err != nil {
		return false, fmt.Errorf("failed to list existing CAS backends: %w", err)
	}

	var hasDefault bool
	for _, b := range backends {
		if b.Default {
			hasDefault = true
			break
		}
	}

	// If there is none, we are done
	if !hasDefault {
		return true, nil
	}

	// Ask the user to confirm the override
	fmt.Println("There is already a default CAS backend in your organization.\nPlease confirm to override y/N: ")
	var gotChallenge string
	fmt.Scanln(&gotChallenge)

	// If the user does not confirm, we are done
	if gotChallenge != "y" && gotChallenge != "Y" {
		return false, nil
	}

	return true, nil
}
