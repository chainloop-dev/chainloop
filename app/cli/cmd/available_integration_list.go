//
// Copyright 2023 The Chainloop Authors.
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

func newAvailableIntegrationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewAvailableIntegrationList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, availableIntegrationListTableOutput)
		},
	}

	return cmd
}

func availableIntegrationListTableOutput(items []*action.AvailableIntegrationItem) error {
	if len(items) == 0 {
		fmt.Println("there are no integrations available")
		return nil
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Version", "Description"})
	for _, i := range items {
		t.AppendRow(table.Row{i.ID, i.Version, i.Description})
		t.AppendSeparator()
	}

	t.Render()

	return nil
}
