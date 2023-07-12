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
	"golang.org/x/exp/maps"
)

func newAttachedIntegrationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List integrations attached to workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewAttachedIntegrationList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, attachedIntegrationListTableOutput)
		},
	}

	return cmd
}

func attachedIntegrationListTableOutput(attachments []*action.AttachedIntegrationItem) error {
	switch n := len(attachments); {
	case n == 0:
		fmt.Println("there are no integrations attached")
		return nil
	case n > 1:
		fmt.Println("Integrations attached to workflows")
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Kind", "Config", "Attached At", "Workflow"})
	for _, attachment := range attachments {
		wf := attachment.Workflow
		integration := attachment.Integration

		// Merge attachment and integration configs to show them in the same table
		// If the same key exists in both configs, the value in attachment config will be used
		if attachment.Config == nil {
			attachment.Config = make(map[string]any)
		}

		if integration.Config == nil {
			integration.Config = make(map[string]any)
		}

		var options []string
		maps.Copy(integration.Config, attachment.Config)

		// Show it as key-value pairs
		for k, v := range integration.Config {
			if v == "" {
				continue
			}
			options = append(options, fmt.Sprintf("%s: %v", k, v))
		}

		t.AppendRow(table.Row{attachment.ID, integration.Kind, strings.Join(options, "\n"), attachment.CreatedAt.Format(time.RFC822), wf.NamespacedName()})
		t.AppendSeparator()
	}

	t.Render()

	return nil
}
