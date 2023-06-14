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
	var integrationID string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := action.NewAvailableIntegrationDescribe(actionOpts).Run(integrationID)
			if err != nil {
				return err
			}

			if item == nil {
				return fmt.Errorf("integration %q not found", integrationID)
			}

			return encodeOutput([]*action.AvailableIntegrationItem{item}, availableIntegrationDescribeTableOutput)
		},
	}

	cmd.Flags().StringVar(&integrationID, "id", "", "integration ID")
	cmd.Flags().BoolVar(&full, "full", false, "show the full output including JSON schemas")
	err := cmd.MarkFlagRequired("id")
	cobra.CheckErr(err)

	return cmd
}

func availableIntegrationDescribeTableOutput(items []*action.AvailableIntegrationItem) error {
	if len(items) != 1 {
		return nil
	}

	i := items[0]

	// General information
	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Version"})
	t.AppendRow(table.Row{i.ID, i.Version})
	t.Render()

	// Schema information
	if err := renderSchemaTable("Registration inputs", i.Registration.Properties); err != nil {
		return err
	}

	if full {
		if err := renderSchemaRaw("Registration JSON schema", i.Registration.Raw); err != nil {
			return err
		}
	}

	if err := renderSchemaTable("Attachment inputs", i.Attachment.Properties); err != nil {
		return err
	}

	if full {
		if err := renderSchemaRaw("Attachment JSON schema", i.Attachment.Raw); err != nil {
			return err
		}
	}

	return nil
}

// render de-normalized schema format
func renderSchemaTable(tableTitle string, properties action.SchemaPropertiesMap) error {
	if len(properties) == 0 {
		return nil
	}

	t := newTableWriter()
	t.SetTitle(tableTitle)
	t.AppendHeader(table.Row{"Field", "Type", "Required", "Description"})

	for k, v := range properties {
		propertyType := v.Type
		if v.Format != "" {
			propertyType = fmt.Sprintf("%s (%s)", propertyType, v.Format)
		}
		t.AppendRow(table.Row{k, propertyType, v.Required, v.Description})
	}

	t.Render()

	return nil
}

// render raw JSON schema document
func renderSchemaRaw(tableTitle string, s string) error {
	var prettyAttachmentJSON bytes.Buffer
	err := json.Indent(&prettyAttachmentJSON, []byte(s), "", "  ")
	if err != nil {
		return err
	}

	rt := newTableWriter()
	rt.SetTitle(tableTitle)
	rt.AppendRow(table.Row{prettyAttachmentJSON.String()})
	rt.Render()

	return nil
}
