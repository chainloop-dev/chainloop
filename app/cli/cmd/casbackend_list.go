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
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
)

func newCASBackendListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List CAS Backends from your organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewCASBackendList(actionOpts).Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, casBackendListTableOutput)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show the full output")
	return cmd
}

func casBackendListTableOutput(backends []*action.CASBackendItem) error {
	if len(backends) == 0 {
		fmt.Println("there are no cas backends associated")
		return nil
	}

	t := newTableWriter()
	header := table.Row{"ID", "Name", "Location", "Provider", "Description", "Limits", "Default"}
	if full {
		header = append(header, "Validation Status", "Created At", "Validated At")
	}

	t.AppendHeader(header)
	for _, b := range backends {
		limits := "no limits"
		if b.Limits != nil {
			limits = fmt.Sprintf("MaxSize: %s", bytefmt.ByteSize(uint64(b.Limits.MaxBytes)))
		}

		r := table.Row{b.ID, b.Name, wrap.String(b.Location, 35), b.Provider, wrap.String(b.Description, 35), limits, b.Default}
		if full {
			r = append(r, b.ValidationStatus,
				b.CreatedAt.Format(time.RFC822),
				b.ValidatedAt.Format(time.RFC822),
			)
		}

		t.AppendRow(r)
		t.AppendSeparator()
	}

	t.Render()

	return nil
}
