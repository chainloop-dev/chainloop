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
	"github.com/spf13/cobra"
)

func newWorkflowContractDeleteCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := action.NewWorkflowContractDelete(actionOpts).Run(name); err != nil {
				return err
			}
			logger.Info().Msg("contract deleted!")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "contract name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	return cmd
}
