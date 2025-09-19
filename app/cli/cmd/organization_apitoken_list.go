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
	"context"
	"fmt"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newAPITokenListCmd() *cobra.Command {
	var (
		includeRevoked bool
		project        string
		scope          string
	)

	var availableScopes = []string{
		"project",
		"global",
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List API tokens in this organization",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if scope != "" && !slices.Contains(availableScopes, scope) {
				return fmt.Errorf("invalid scope %q, please chose one of: %v", scope, availableScopes)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewAPITokenList(actionOpts).Run(context.Background(), includeRevoked, project, scope)
			if err != nil {
				return fmt.Errorf("listing API tokens: %w", err)
			}

			return output.EncodeOutput(flagOutputFormat, res, apiTokenListTableOutput)
		},
	}

	cmd.Flags().BoolVarP(&includeRevoked, "all", "a", false, "show all API tokens including revoked ones")
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project name")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", fmt.Sprintf("filter by scope, available scopes: %v", availableScopes))
	return cmd
}
