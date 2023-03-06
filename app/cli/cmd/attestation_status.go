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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
)

var full bool

func newAttestationStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "check the status of the current attestation process",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := action.NewAttestationStatus(
				&action.AttestationStatusOpts{
					ActionsOpts: actionOpts,
				},
			)

			res, err := a.Run()
			if err != nil {
				return err
			}

			return encodeOutput(res, attestationStatusTableOutput)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "full report including current recorded values")

	return cmd
}

func attestationStatusTableOutput(status *action.AttestationStatusResult) error {
	// General info table
	gt := newTableWriter()
	gt.AppendRow(table.Row{"Initialized At", status.InitializedAt.Format(time.RFC822)})
	gt.AppendSeparator()
	meta := status.WorkflowMeta
	gt.AppendRow(table.Row{"Workflow", meta.WorkflowID})
	gt.AppendRow(table.Row{"Name", meta.Name})
	gt.AppendRow(table.Row{"Team", meta.Team})
	gt.AppendRow(table.Row{"Project", meta.Project})
	gt.AppendRow(table.Row{"Contract Revision", meta.ContractRevision})
	if status.RunnerContext.JobURL != "" {
		gt.AppendRow(table.Row{"Runner Type", status.RunnerContext.RunnerType})
		gt.AppendRow(table.Row{"Runner URL", status.RunnerContext.JobURL})
	}
	gt.Render()

	if err := materialsTable(status); err != nil {
		return err
	}

	if err := envVarsTable(status); err != nil {
		return err
	}

	if status.DryRun {
		colors := text.Colors{text.FgHiBlack, text.BgHiYellow}
		fmt.Println(colors.Sprint("The attestation is being crafted in dry-run mode. It will not get stored once rendered"))
	}
	return nil
}

func envVarsTable(status *action.AttestationStatusResult) error {
	if len(status.EnvVars) == 0 && len(status.RunnerContext.EnvVars) == 0 {
		return nil
	}

	if len(status.EnvVars) > 0 {
		// Env Variables table
		evt := newTableWriter()
		evt.SetTitle("Env Variables")
		for k, v := range status.EnvVars {
			if v == "" {
				v = "NOT FOUND"
			}
			evt.AppendRow(table.Row{k, v})
		}
		evt.Render()
	}

	runnerVars := status.RunnerContext.EnvVars
	if len(runnerVars) > 0 && full {
		evt := newTableWriter()
		evt.SetTitle("Runner context")
		for k, v := range runnerVars {
			if v == "" {
				v = "NOT FOUND"
			}
			evt.AppendRow(table.Row{k, v})
		}
		evt.Render()
	}

	return nil
}
func materialsTable(status *action.AttestationStatusResult) error {
	if len(status.Materials) == 0 {
		return nil
	}

	mt := newTableWriter()
	mt.SetTitle("Materials")

	header := table.Row{"Name", "Type", "Set", "Required", "Is output"}
	if full {
		header = append(header, "Value")
	}

	mt.AppendHeader(header)
	for _, m := range status.Materials {
		row := table.Row{m.Name, m.Type, hBool(m.Set), hBool(m.Required)}
		var outputInfo string
		if m.IsOutput {
			outputInfo = "x"
		}

		row = append(row, outputInfo)
		if full {
			row = append(row, m.Value)
		}

		mt.AppendRow(row)
	}
	mt.Render()

	return nil
}

func hBool(b bool) string {
	if b {
		return "Yes"
	}

	return "No"
}
