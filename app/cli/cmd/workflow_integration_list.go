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

func newWorkflowIntegrationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List integrations attached to workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowIntegrationList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, integrationAttachmentListTableOutput)
		},
	}

	return cmd
}

func integrationAttachmentListTableOutput(attachments []*action.IntegrationAttachmentItem) error {
	if len(attachments) == 0 {
		fmt.Println("there are no integration attached")
		return nil
	}

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Kind", "Config", "Attached At", "Workflow"})
	for _, i := range attachments {
		wf := i.Workflow
		integration := i.Integration

		maps.Copy(i.Config, integration.Config)
		var options []string
		for k, v := range i.Config {
			if v == "" {
				continue
			}
			options = append(options, fmt.Sprintf("%s: %v", k, v))
		}
		t.AppendRow(table.Row{i.ID, integration.Name, strings.Join(options, "\n"), i.CreatedAt.Format(time.RFC822), wf.NamespacedName()})
		t.AppendSeparator()
	}

	t.Render()

	return nil
}
