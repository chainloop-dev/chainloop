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
	"context"
	"fmt"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newAPITokenListCmd() *cobra.Command {
	var (
		includeRevoked bool
		statusFilter   string
		project        string
		scope          string
	)

	var availableScopes = []string{
		"project",
		"global",
	}

	var availableStatusFilters = []string{
		"active",
		"revoked",
		"all",
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List API tokens in this organization",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if scope != "" && !slices.Contains(availableScopes, scope) {
				return fmt.Errorf("invalid scope %q, please chose one of: %v", scope, availableScopes)
			}

			if statusFilter != "" && !slices.Contains(availableStatusFilters, statusFilter) {
				return fmt.Errorf("invalid status %q, please choose one of: %v", statusFilter, availableStatusFilters)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// --all is deprecated: map it to --status all
			if includeRevoked {
				cmd.PrintErr("Warning: --all is deprecated, use --status all instead\n")
				if statusFilter == "" {
					statusFilter = "all"
				}
			}

			res, err := action.NewAPITokenList(ActionOpts).Run(context.Background(), statusFilter, project, scope)
			if err != nil {
				return fmt.Errorf("listing API tokens: %w", err)
			}

			return output.EncodeOutput(flagOutputFormat, res, apiTokenListTableOutput)
		},
	}

	cmd.Flags().BoolVarP(&includeRevoked, "all", "a", false, "Deprecated: use --status all instead")
	if err := cmd.Flags().MarkDeprecated("all", "use --status all instead"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", fmt.Sprintf("filter by token status, available values: %v", availableStatusFilters))
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project name")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", fmt.Sprintf("filter by scope, available scopes: %v", availableScopes))
	return cmd
}
