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
	"fmt"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/jedib0t/go-pretty/v6/table"
)

type renderer struct {
	render func(t table.Writer) string
	format string
}

type RenderOpt func(r *renderer) error

func WithFormat(format string) RenderOpt {
	return func(r *renderer) error {
		switch format {
		case "text":
			r.render = func(t table.Writer) string {
				return t.Render()
			}
		case "markdown":
			r.render = func(t table.Writer) string {
				return t.RenderMarkdown()
			}
		case "html":
			r.render = func(t table.Writer) string {
				return t.RenderHTML()
			}
		default:
			return fmt.Errorf("unsupported format %s", format)
		}

		r.format = format
		return nil
	}
}

func newRenderer(opts ...RenderOpt) (*renderer, error) {
	r := &renderer{
		render: func(t table.Writer) string {
			return t.Render()
		}, format: "text",
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *renderer) summaryTable(m *ChainloopMetadata, predicate chainloop.NormalizablePredicate) (string, error) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)

	if m == nil || m.Workflow == nil {
		return "", fmt.Errorf("workflow metadata is missing")
	}

	if predicate == nil {
		return "", fmt.Errorf("predicate is nil")
	}

	tw.SetTitle("Workflow")
	tw.AppendRow(table.Row{"ID", m.Workflow.ID})
	tw.AppendRow(table.Row{"Name", m.Workflow.Name})
	tw.AppendRow(table.Row{"Team", m.Workflow.Team})
	tw.AppendRow(table.Row{"Project", m.Workflow.Project})
	tw.AppendSeparator()

	wr := m.WorkflowRun
	if wr == nil {
		return "", fmt.Errorf("workflow run metadata is missing")
	}

	tw.AppendRow(table.Row{"Workflow Run"})
	tw.AppendSeparator()
	tw.AppendRow(table.Row{"ID", wr.ID})
	tw.AppendRow(table.Row{"Started At", wr.StartedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"Finished At", wr.FinishedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"State", wr.State})
	tw.AppendRow(table.Row{"Runner Link", wr.RunURL})

	var result = r.render(tw)

	// Materials
	materials := predicate.GetMaterials()
	if len(materials) > 0 {
		mt := table.NewWriter()
		mt.SetStyle(table.StyleLight)

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

		result += "\n" + r.render(mt)
	}

	// Env variables
	envVars := predicate.GetEnvVars()
	if len(envVars) > 0 {
		mt := table.NewWriter()
		mt.SetStyle(table.StyleLight)
		mt.SetTitle("Environment Variables")

		header := table.Row{"Name", "Value"}
		mt.AppendHeader(header)

		// sort env vars by name
		var keys []string
		for k := range envVars {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			v := envVars[k]
			mt.AppendRow(table.Row{k, v})
		}

		result += "\n" + r.render(mt)
	}

	result += fmt.Sprintf("\n\nGet Full Attestation\n\n$ chainloop workflow run describe --id %s -o statement", wr.ID)

	return result, nil
}

func SummaryTable(req *ExecutionRequest, opts ...RenderOpt) (string, error) {
	renderer, err := newRenderer(opts...)
	if err != nil {
		return "", err
	}

	return renderer.summaryTable(req.ChainloopMetadata, req.Input.Attestation.Predicate)
}
