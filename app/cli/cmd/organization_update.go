//
// Copyright 2023-2025 The Chainloop Authors.
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

func newOrganizationUpdateCmd() *cobra.Command {
	var (
		orgName                  string
		blockOnPolicyViolation   bool
		policiesAllowedHostnames []string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing organization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := &action.NewOrgUpdateOpts{}
			if cmd.Flags().Changed("block") {
				opts.BlockOnPolicyViolation = &blockOnPolicyViolation
			}

			if cmd.Flags().Changed("policies-allowed-hostnames") {
				opts.PoliciesAllowedHostnames = &policiesAllowedHostnames
			}

			_, err := action.NewOrgUpdate(actionOpts).Run(cmd.Context(), orgName, opts)
			if err != nil {
				return err
			}

			logger.Info().Msg("Organization updated!")
			return nil
		},
	}

	cmd.Flags().StringVar(&orgName, "name", "", "organization name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	cmd.Flags().BoolVar(&blockOnPolicyViolation, "block", false, "set the default policy violation blocking strategy")
	cmd.Flags().StringSliceVar(&policiesAllowedHostnames, "policies-allowed-hostnames", []string{}, "set the allowed hostnames for the policy engine")
	return cmd
}
