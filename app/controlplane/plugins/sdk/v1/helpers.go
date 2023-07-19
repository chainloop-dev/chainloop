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

package sdk

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

func SummaryTable(req *ExecutionRequest) []byte {
	buffer := bytes.NewBuffer(nil)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.SetOutputMirror(buffer)

	tw.SetTitle("Workflow")
	m := req.ChainloopMetadata
	tw.AppendRow(table.Row{"ID", m.Workflow.ID})
	tw.AppendRow(table.Row{"Name", m.Workflow.Name})
	tw.AppendRow(table.Row{"Team", m.Workflow.Team})
	tw.AppendRow(table.Row{"Project", m.Workflow.Project})
	tw.AppendSeparator()

	wr := m.WorkflowRun
	tw.AppendRow(table.Row{"Workflow Run"})
	tw.AppendSeparator()
	tw.AppendRow(table.Row{"ID", wr.ID})
	tw.AppendRow(table.Row{"Started At", wr.StartedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"Finished At", wr.FinishedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"State", wr.State})
	tw.AppendRow(table.Row{"Runner Link", wr.RunURL})

	var result = tw.Render()

	predicate := req.Input.Attestation.Predicate
	// Materials
	materials := predicate.GetMaterials()
	if len(materials) > 0 {
		mt := table.NewWriter()
		mt.SetStyle(table.StyleLight)
		mt.SetOutputMirror(buffer)

		mt.SetTitle("Materials")
		mt.AppendHeader(table.Row{"Name", "Type", "Value"})

		for _, m := range materials {
			// Initialize simply with the value
			displayValue := m.Value
			// Override if there is a hash attached
			if m.Hash != nil {
				name := m.Value
				if m.EmbeddedInline || m.UploadedToCAS {
					name = m.Filename
				}

				displayValue = fmt.Sprintf("%s@%s", name, m.Hash)
			}

			row := table.Row{m.Name, m.Type, displayValue}
			mt.AppendRow(row)
		}

		result += "\n" + mt.Render()
	}

	// Env variables
	envVars := predicate.GetEnvVars()
	if len(envVars) > 0 {
		mt := table.NewWriter()
		mt.SetStyle(table.StyleLight)
		mt.SetOutputMirror(buffer)
		mt.SetTitle("Environment Variables")

		header := table.Row{"Name", "Value"}
		mt.AppendHeader(header)
		for k, v := range envVars {
			mt.AppendRow(table.Row{k, v})
		}

		result += "\n" + mt.Render()
	}

	result += fmt.Sprintf("\n\nGet full Attestation\n\n- chainloop workflow run describe --id %s -o statement", wr.ID)

	return []byte(result)
}
