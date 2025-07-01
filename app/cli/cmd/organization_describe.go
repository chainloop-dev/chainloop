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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newOrganizationDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"current-context"},
		Short:   "Describe the current organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewConfigCurrentContext(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, contextTableOutput)
		},
	}

	return cmd
}

func contextTableOutput(config *action.ConfigContextItem) error {
	gt := newTableWriter()
	gt.SetTitle("Current Context")
	gt.AppendRow(table.Row{"Logged in as", config.CurrentUser.PrintUserProfileWithEmail()})
	gt.AppendSeparator()

	if m := config.CurrentMembership; m != nil {
		gt.AppendRow(table.Row{"Organization", fmt.Sprintf("%s (role=%s)\nPolicy strategy=%s", m.Org.Name, m.Role, m.Org.PolicyViolationBlockingStrategy)})
	}

	backend := config.CurrentCASBackend
	if backend != nil {
		gt.AppendSeparator()
		gt.AppendRow(table.Row{"Default CAS Backend", fmt.Sprintf("%s (provider=%s, status=%q)", backend.Location, backend.Provider, backend.ValidationStatus)})
	}

	gt.Render()
	return nil
}
