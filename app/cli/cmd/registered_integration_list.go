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
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newRegisteredIntegrationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewRegisteredIntegrationList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, registeredIntegrationListTableOutput)
		},
	}

	return cmd
}

func registeredIntegrationListTableOutput(items []*action.RegisteredIntegrationItem) error {
	switch n := len(items); {
	case n == 0:
		fmt.Println("there are no third party integrations configured in your organization yet")
		return nil
	case n > 1:
		fmt.Println("Integrations registered and configured in your organization")
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Name", "Description", "Kind", "Config", "Created At"})
	for _, i := range items {
		var options []string
		for k, v := range i.Config {
			options = append(options, fmt.Sprintf("%s: %v", k, v))
		}
		t.AppendRow(table.Row{i.ID, i.Name, i.Description, i.Kind, strings.Join(options, "\n"), i.CreatedAt.Format(time.RFC822)})
		t.AppendSeparator()
	}

	t.Render()

	return nil
}
