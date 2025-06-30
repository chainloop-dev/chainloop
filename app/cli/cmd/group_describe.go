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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newGroupDescribeCmd() *cobra.Command {
	var groupName string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Get detailed information about a specific group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			group, err := action.NewGroupDescribe(actionOpts).Run(cmd.Context(), groupName)
			if err != nil {
				return fmt.Errorf("describing group: %w", err)
			}

			// Print the group details
			if err := encodeOutput(group, groupItemTableOutput); err != nil {
				return fmt.Errorf("failed to print group: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&groupName, "name", "", "Name of the group to describe")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	return cmd
}
