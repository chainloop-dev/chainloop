//
// Copyright 2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply resources from YAML files",
		Long: `Apply resources from a YAML file or directory.
Supports multi-document YAML files. Each document must have a 'kind' field.`,
		Example: `  # Apply resources from a single file
  chainloop apply -f my-contract.yaml

  # Apply resources from a directory
  chainloop apply -f ./contracts/`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			results, err := action.NewApply(ActionOpts).Run(cmd.Context(), filePath)
			if err != nil {
				return err
			}

			for _, r := range results {
				status := "unchanged"
				if r.Changed {
					status = "applied"
				}
				logger.Info().Msgf("%s/%s %s", r.Kind, r.Name, status)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "path to a YAML file or directory")
	cobra.CheckErr(cmd.MarkFlagRequired("file"))

	return cmd
}
