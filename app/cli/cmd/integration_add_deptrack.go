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
	"syscall"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newIntegrationAddDepTrackCmd() *cobra.Command {
	var instance, integrationDescription string
	var allowAutoCreate bool

	cmd := &cobra.Command{
		Use:        "dependency-track",
		Aliases:    []string{"deptrack"},
		Short:      "Add Dependency-Track integration ",
		Deprecated: "use `chainloop integration add` instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Enter API Token: \n")
			apiKey, err := term.ReadPassword(syscall.Stdin)
			if err != nil {
				return fmt.Errorf("retrieving token from stdin: %w", err)
			}

			opts := map[string]any{
				"instanceURI":     instance,
				"apiKey":          string(apiKey),
				"allowAutoCreate": allowAutoCreate,
			}

			res, err := action.NewIntegrationAdd(actionOpts).Run("dependencytrack", integrationDescription, opts)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.IntegrationItem{res}, integrationListTableOutput)
		},
	}

	cmd.Flags().StringVar(&instance, "instance", "", "dependency track instance URL")
	cobra.CheckErr(cmd.MarkFlagRequired("instance"))

	cmd.Flags().BoolVar(&allowAutoCreate, "allow-project-auto-create", false, "Allow auto-creation of projects or require to always specify an existing one")

	return cmd
}
