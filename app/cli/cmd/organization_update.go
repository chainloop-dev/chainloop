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
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newOrganizationUpdateCmd() *cobra.Command {
	var (
		orgName                         string
		blockOnPolicyViolation          bool
		policiesAllowedHostnames        []string
		preventImplicitWorkflowCreation bool
		restrictContractCreation        bool
		apiTokenInactivityThreshold     string
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

			if cmd.Flags().Changed("prevent-implicit-workflow-creation") {
				opts.PreventImplicitWorkflowCreation = &preventImplicitWorkflowCreation
			}

			if cmd.Flags().Changed("restrict-contract-creation") {
				opts.RestrictContractCreation = &restrictContractCreation
			}

			if cmd.Flags().Changed("api-token-inactivity-threshold") {
				if apiTokenInactivityThreshold == "0" {
					// Disable by setting duration to zero
					d := time.Duration(0)
					opts.APITokenInactivityThreshold = &d
				} else {
					d, err := time.ParseDuration(apiTokenInactivityThreshold)
					if err != nil {
						return fmt.Errorf("invalid duration %q: %w", apiTokenInactivityThreshold, err)
					}
					if d < 24*time.Hour {
						return fmt.Errorf("api-token-inactivity-threshold must be at least 24h (1 day)")
					}
					opts.APITokenInactivityThreshold = &d
				}
			}

			_, err := action.NewOrgUpdate(ActionOpts).Run(cmd.Context(), orgName, opts)
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
	cmd.Flags().BoolVar(&preventImplicitWorkflowCreation, "prevent-implicit-workflow-creation", false, "prevent workflows and projects from being created implicitly during attestation init")
	cmd.Flags().BoolVar(&restrictContractCreation, "restrict-contract-creation", false, "restrict contract creation (org-level and project-level) to only organization admins (owner/admin roles)")
	cmd.Flags().StringVar(&apiTokenInactivityThreshold, "api-token-inactivity-threshold", "", "auto-revoke API tokens inactive for this duration (e.g. '2160h' for 90 days, '0' to disable)")
	return cmd
}
