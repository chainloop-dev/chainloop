//
// Copyright 2024 The Chainloop Authors.
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
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
)

var full bool

func newAttestationStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "check the status of the current attestation process",
		Annotations: map[string]string{
			useWorkflowRobotAccount: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := action.NewAttestationStatus(
				&action.AttestationStatusOpts{
					ActionsOpts: actionOpts,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to load action: %w", err)
			}

			res, err := a.Run(cmd.Context(), attestationID)
			if err != nil {
				return err
			}

			return encodeOutput(res, simpleStatusTable)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "full report including current recorded values")
	flagAttestationID(cmd)

	return cmd
}

func simpleStatusTable(status *action.AttestationStatusResult) error {
	return attestationStatusTableOutput(status, false)
}

func fullStatusTable(status *action.AttestationStatusResult) error {
	return attestationStatusTableOutput(status, true)
}

func attestationStatusTableOutput(status *action.AttestationStatusResult, full bool) error {
	// General info table
	gt := newTableWriter()
	gt.AppendRow(table.Row{"Initialized At", status.InitializedAt.Format(time.RFC822)})
	gt.AppendSeparator()
	meta := status.WorkflowMeta
	gt.AppendRow(table.Row{"Attestation ID", status.AttestationID})
	gt.AppendRow(table.Row{"Name", meta.Name})
	gt.AppendRow(table.Row{"Team", meta.Team})
	gt.AppendRow(table.Row{"Project", meta.Project})
	gt.AppendRow(table.Row{"Contract Revision", meta.ContractRevision})
	if status.RunnerContext.JobURL != "" {
		gt.AppendRow(table.Row{"Runner Type", status.RunnerContext.RunnerType})
		gt.AppendRow(table.Row{"Runner URL", status.RunnerContext.JobURL})
	}

	if len(status.Annotations) > 0 {
		gt.AppendRow(table.Row{"Annotations", "------"})
		for _, a := range status.Annotations {
			value := a.Value
			if value == "" {
				value = "[NOT SET]"
			}
			gt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	gt.Render()

	if err := materialsTable(status, full); err != nil {
		return err
	}

	if err := envVarsTable(status, full); err != nil {
		return err
	}

	if status.DryRun {
		colors := text.Colors{text.FgHiBlack, text.BgHiYellow}
		fmt.Println(colors.Sprint("The attestation is being crafted in dry-run mode. It will not get stored once rendered"))
	}
	return nil
}

func envVarsTable(status *action.AttestationStatusResult, full bool) error {
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
func materialsTable(status *action.AttestationStatusResult, full bool) error {
	if len(status.Materials) == 0 {
		return nil
	}

	mt := newTableWriter()
	mt.SetTitle("Materials")

	for _, m := range status.Materials {
		mt.AppendRow(table.Row{"Name", m.Name})
		mt.AppendRow(table.Row{"Type", m.Type})
		mt.AppendRow(table.Row{"Set", hBool(m.Set)})
		mt.AppendRow(table.Row{"Required", hBool(m.Required)})
		if m.IsOutput {
			mt.AppendRow(table.Row{"Is output", "Yes"})
		}

		if full {
			if m.Value != "" {
				v := m.Value
				if m.Tag != "" {
					v = fmt.Sprintf("%s:%s", v, m.Tag)
				}
				mt.AppendRow(table.Row{"Value", wrap.String(v, 100)})
			}

			if m.Hash != "" {
				mt.AppendRow(table.Row{"Digest", m.Hash})
			}
		}

		if len(m.Annotations) > 0 {
			mt.AppendRow(table.Row{"Annotations", "------"})
			for _, a := range m.Annotations {
				value := a.Value
				if value == "" {
					value = "[NOT SET]"
				}

				mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
			}
		}

		mt.AppendSeparator()
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
