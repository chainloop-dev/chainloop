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
	"github.com/muesli/reflow/wrap"
)

type renderer struct {
	render  func(t table.Writer) string
	maxSize int
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

		return nil
	}
}

func WithMaxSize(max int) RenderOpt {
	return func(r *renderer) error {
		r.maxSize = max
		return nil
	}
}

func newRenderer(opts ...RenderOpt) (*renderer, error) {
	r := &renderer{
		render: func(t table.Writer) string {
			return t.Render()
		},
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
	tw.AppendRow(table.Row{"Attestation", wr.AttestationDigest})
	tw.AppendRow(table.Row{"Started At", wr.StartedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"Finished At", wr.FinishedAt.Format(time.RFC822)})
	tw.AppendRow(table.Row{"State", wr.State})
	tw.AppendRow(table.Row{"Runner Link", wr.RunURL})
	if annotations := predicate.GetAnnotations(); len(annotations) > 0 {
		keys := make([]string, 0, len(annotations))
		for k := range annotations {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		tw.AppendRow(table.Row{"Annotations", "------"})
		for _, k := range keys {
			tw.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", k, annotations[k])})
		}
	}

	var result = r.render(tw)

	// Materials
	materials := predicate.GetMaterials()
	if len(materials) > 0 {
		mt := table.NewWriter()
		mt.SetStyle(table.StyleLight)

		mt.SetTitle("Materials")
		for _, m := range materials {
			mt.AppendRow(table.Row{"Name", m.Name})
			mt.AppendRow(table.Row{"Type", m.Type})
			value := m.Value
			// Override the value for the filename of the item uploaded
			if m.EmbeddedInline || m.UploadedToCAS {
				value = m.Filename
			}
			mt.AppendRow(table.Row{"Value", wrap.String(value, 100)})
			if m.Hash != nil {
				mt.AppendRow(table.Row{"Digest", m.Hash.String()})
			}

			if annotations := m.Annotations; len(annotations) > 0 {
				keys := make([]string, 0, len(annotations))
				for k := range annotations {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				mt.AppendRow(table.Row{"Annotations", "------"})
				for _, k := range keys {
					mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", k, annotations[k])})
				}
			}
			mt.AppendSeparator()
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

	footer := fmt.Sprintf("\n\nGet Full Attestation\n\n$ chainloop workflow run describe --id %s -o statement", wr.ID)

	// Truncate the text if it's too long to be displayed, the footer will be kept
	if r.maxSize > 0 {
		result = truncateText(result, r.maxSize-len(footer))
	}

	result += footer

	return result, nil
}

// Truncate returns the first n runes of s.
func truncateText(s string, n int) string {
	truncatedPrefix := "... (truncated)"
	n -= len(truncatedPrefix)

	if len(s) <= n {
		return s
	}
	for i := range s {
		if n == 0 {
			return s[:i] + truncatedPrefix
		}
		n--
	}
	return s
}

func SummaryTable(req *ExecutionRequest, opts ...RenderOpt) (string, error) {
	renderer, err := newRenderer(opts...)
	if err != nil {
		return "", err
	}

	return renderer.summaryTable(req.ChainloopMetadata, req.Input.Attestation.Predicate)
}
