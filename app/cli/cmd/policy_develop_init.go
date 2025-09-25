//
// Copyright 2025 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newPolicyDevelopInitCmd() *cobra.Command {
	var (
		force       bool
		embedded    bool
		name        string
		description string
		directory   string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new policy",
		Long: `Initialize a new policy by creating template policy files in the specified directory.
By default, it creates chainloop-policy.yaml and chainloop-policy.rego files.`,
		Example: `  
  # Initialize in current directory with separate files
  chainloop policy develop init

  # Initialize in specific directory with embedded format and policy name
  chainloop policy develop init --directory ./policies --embedded --name mypolicy`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if directory == "" {
				directory = "."
			}
			opts := &action.PolicyInitOpts{
				Force:       force,
				Embedded:    embedded,
				Name:        name,
				Description: description,
				Directory:   directory,
			}

			policyInit, err := action.NewPolicyInit(opts, ActionOpts)
			if err != nil {
				return fmt.Errorf("failed to initialize policy: %w", err)
			}

			if err := policyInit.Run(); err != nil {
				return newGracefulError(err)
			}

			logger.Info().Msg("Initialized policy files")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	cmd.Flags().BoolVar(&embedded, "embedded", false, "initialize an embedded policy (single YAML file)")
	cmd.Flags().StringVar(&name, "name", "", "name of the policy")
	cmd.Flags().StringVar(&description, "description", "", "description of the policy")
	cmd.Flags().StringVar(&directory, "directory", "", "directory for policy")
	return cmd
}
