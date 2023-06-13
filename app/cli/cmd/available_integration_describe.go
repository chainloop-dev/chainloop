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
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func newAvailableIntegrationDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := action.NewAvailableIntegrationList(actionOpts).Run()
			if err != nil {
				return err
			}

			// Filter by ID
			wantID, err := cmd.Flags().GetString("id")
			if err != nil {
				return err
			}

			for _, i := range items {
				if i.ID == wantID {
					return encodeOutput([]*action.AvailableIntegrationItem{i}, availableIntegrationDescribeTableOutput)
				}
			}

			return fmt.Errorf("integration %q not found", wantID)
		},
	}

	cmd.Flags().String("id", "", "integration ID")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	return cmd
}

func availableIntegrationDescribeTableOutput(items []*action.AvailableIntegrationItem) error {
	if len(items) != 1 {
		return nil
	}

	i := items[0]

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Version"})
	t.AppendRow(table.Row{i.ID, i.Version})
	t.Render()

	var prettyRegistrationJSON bytes.Buffer
	err := json.Indent(&prettyRegistrationJSON, []byte(i.RegistrationJSONSchema), "", "  ")
	if err != nil {
		return err
	}

	rt := newTableWriter()
	rt.SetTitle("Registration Schema")
	rt.AppendRow(table.Row{prettyRegistrationJSON.String()})
	rt.Render()

	var prettyAttachmentJSON bytes.Buffer
	err = json.Indent(&prettyAttachmentJSON, []byte(i.AttachmentJSONSchema), "", "  ")
	if err != nil {
		return err
	}

	at := newTableWriter()
	at.SetTitle("Attachment schema")
	at.AppendRow(table.Row{prettyAttachmentJSON.String()})
	at.Render()

	return nil
}
