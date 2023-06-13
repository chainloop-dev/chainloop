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
	"github.com/santhosh-tekuri/jsonschema/v5"
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

	t := newTableWriter()
	t.AppendHeader(table.Row{"ID", "Version"})
	t.AppendRow(table.Row{i.ID, i.Version})
	t.Render()

	rt := newTableWriter()
	rt.SetTitle("Registration inputs")
	rt.AppendHeader(table.Row{"Field", "Type", "Required", "Description"})
	if err := renderSchemaOptions(rt, i.Registration.Parsed); err != nil {
		return err
	}
	rt.Render()

	if full {
		var prettyRegistrationJSON bytes.Buffer
		err := json.Indent(&prettyRegistrationJSON, []byte(i.Registration.Raw), "", "  ")
		if err != nil {
			return err
		}

		rt = newTableWriter()
		rt.SetTitle("Registration JSON schema")
		rt.AppendRow(table.Row{prettyRegistrationJSON.String()})
		rt.Render()
	}

	rt = newTableWriter()
	rt.SetTitle("Attachment inputs")
	rt.AppendHeader(table.Row{"Field", "Type", "Required", "Description"})
	if err := renderSchemaOptions(rt, i.Attachment.Parsed); err != nil {
		return err
	}
	rt.Render()

	if full {
		var prettyAttachmentJSON bytes.Buffer
		err := json.Indent(&prettyAttachmentJSON, []byte(i.Attachment.Raw), "", "  ")
		if err != nil {
			return err
		}
		rt = newTableWriter()
		rt.SetTitle("Attachment JSON schema")
		rt.AppendRow(table.Row{prettyAttachmentJSON.String()})
		rt.Render()
	}

	return nil
}

func renderSchemaOptions(t table.Writer, s *jsonschema.Schema) error {
	// Schema with reference
	if s.Ref != nil {
		return renderSchemaOptions(t, s.Ref)
	}

	// Appended schemas
	if s.AllOf != nil {
		for _, s := range s.AllOf {
			if err := renderSchemaOptions(t, s); err != nil {
				return err
			}
		}
	}

	if s.Properties != nil {
		requiredMap := make(map[string]bool)
		for _, r := range s.Required {
			requiredMap[r] = true
		}

		for k, v := range s.Properties {
			if err := renderSchemaOptions(t, v); err != nil {
				return err
			}

			// We do not support nested schemas
			// They are restricted at build time
			var required = requiredMap[k]
			t.AppendRow(table.Row{k, v.Types[0], required, v.Description})
		}
	}

	return nil
}
