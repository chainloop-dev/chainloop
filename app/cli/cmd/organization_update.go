//
// Copyright 2023-2026 The Chainloop Authors.
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
	"math"
	"strconv"

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
		apiTokenMaxDaysInactive         string
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

			if cmd.Flags().Changed("api-token-max-days-inactive") {
				days, err := strconv.Atoi(apiTokenMaxDaysInactive)
				if err != nil {
					return fmt.Errorf("invalid value %q: must be a number of days (0 to disable)", apiTokenMaxDaysInactive)
				}
				if days < 0 || days > math.MaxInt32 {
					return fmt.Errorf("api-token-max-days-inactive must be between 0 (disabled) and %d", math.MaxInt32)
				}
				opts.APITokenMaxDaysInactive = &days
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
	cmd.Flags().StringVar(&apiTokenMaxDaysInactive, "api-token-max-days-inactive", "", "maximum days of inactivity before API tokens are auto-revoked (e.g. '90', '0' to disable)")
	return cmd
}
