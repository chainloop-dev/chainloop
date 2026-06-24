//
// Copyright 2024-2026 The Chainloop Authors.
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
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
)

var full bool

func newAttestationStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "check the status of the current attestation process",
		Annotations: map[string]string{
			useAPIToken:                     "true",
			supportsFederatedAuthAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := action.NewAttestationStatus(
				&action.AttestationStatusOpts{
					UseAttestationRemoteState: attestationID != "",
					ActionsOpts:               ActionOpts,
					LocalStatePath:            attestationLocalStatePath,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to load action: %w", err)
			}

			res, err := a.Run(cmd.Context(), attestationID)
			if err != nil {
				return err
			}

			outputF := simpleStatusTable
			if full {
				outputF = fullStatusTable
			}

			return output.EncodeOutput(flagOutputFormat, res, outputF)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "full report including current recorded values")
	flagAttestationID(cmd)

	return cmd
}

func simpleStatusTable(status *action.AttestationStatusResult) error {
	return attestationStatusTableOutput(status, os.Stdout, false)
}

func fullStatusTable(status *action.AttestationStatusResult) error {
	return attestationStatusTableOutput(status, os.Stdout, true)
}

func fullStatusTableWithWriter(status *action.AttestationStatusResult, w io.Writer) error {
	return attestationStatusTableOutput(status, w, true)
}

func attestationStatusTableOutput(status *action.AttestationStatusResult, w io.Writer, full bool) error {
	// General info table
	gt := output.NewTableWriterWithWriter(w)
	gt.AppendRow(table.Row{"Initialized At", status.InitializedAt.Format(time.RFC822)})
	gt.AppendSeparator()
	meta := status.WorkflowMeta
	gt.AppendRow(table.Row{"Attestation ID", status.AttestationID})
	if status.Digest != "" {
		gt.AppendRow(table.Row{"Digest", status.Digest})
	}
	gt.AppendRow(table.Row{"Organization", meta.Organization})
	gt.AppendRow(table.Row{"Name", meta.Name})
	gt.AppendRow(table.Row{"Project", meta.Project})
	projectVersion := versionStringAttestation(meta.ProjectVersion, status.IsPushed)
	gt.AppendRow(table.Row{"Version", projectVersion})
	gt.AppendRow(table.Row{"Contract", fmt.Sprintf("%s (revision %s)", meta.ContractName, meta.ContractRevision)})
	if status.RunnerContext.JobURL != "" {
		gt.AppendRow(table.Row{"Runner Type", status.RunnerContext.RunnerType})
		gt.AppendRow(table.Row{"Runner URL", status.RunnerContext.JobURL})
	}

	if len(status.Annotations) > 0 {
		gt.AppendRow(table.Row{"Annotations", "------"})
		for _, a := range status.Annotations {
			value := a.Value
			if value == "" {
				value = NotSet
			}
			gt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	if status.TimestampAuthority != "" {
		gt.AppendRow(table.Row{"Timestamp Authority", status.TimestampAuthority})
	}

	var blockingColor text.Color
	var blockingText = action.PolicyViolationBlockingStrategyAdvisory
	if status.MustBlockOnPolicyViolations {
		blockingColor = text.FgHiYellow
		blockingText = action.PolicyViolationBlockingStrategyEnforced
	}
	gt.AppendRow(table.Row{"Policy violation strategy", blockingColor.Sprint(blockingText)})

	evs := status.PolicyEvaluations[chainloop.AttPolicyEvaluation]
	if len(evs) > 0 {
		gt.AppendRow(table.Row{"Policies", "------"})
		policiesTable(evs, gt, flagDebug)
	}

	// Add the Attestation View URL if available
	if status.AttestationViewURL != "" {
		gt.AppendRow(table.Row{"Attestation View URL", status.AttestationViewURL})
	}

	gt.Render()

	if err := materialsTable(status, w, full); err != nil {
		return err
	}

	return envVarsTable(status, w, full)
}

func envVarsTable(status *action.AttestationStatusResult, w io.Writer, full bool) error {
	if len(status.EnvVars) == 0 && len(status.RunnerContext.EnvVars) == 0 {
		return nil
	}

	if len(status.EnvVars) > 0 {
		// Env Variables table
		evt := output.NewTableWriterWithWriter(w)
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
		evt := output.NewTableWriterWithWriter(w)
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
func materialsTable(status *action.AttestationStatusResult, w io.Writer, full bool) error {
	if len(status.Materials) == 0 {
		return nil
	}

	// Partition materials into standalone (ungrouped) ones and choke groups.
	// Grouped materials are rendered together under a group header so it is
	// clear they form an "at least one of" set rather than independent materials.
	var ungrouped []action.AttestationStatusMaterial
	groupedBy := make(map[string][]action.AttestationStatusMaterial)
	var groupOrder []string
	for _, m := range status.Materials {
		if m.Group == "" {
			ungrouped = append(ungrouped, m)
			continue
		}
		if _, ok := groupedBy[m.Group]; !ok {
			groupOrder = append(groupOrder, m.Group)
		}
		groupedBy[m.Group] = append(groupedBy[m.Group], m)
	}

	byName := func(a, b action.AttestationStatusMaterial) int { return strings.Compare(a.Name, b.Name) }
	slices.SortFunc(ungrouped, byName)
	slices.Sort(groupOrder)

	mt := output.NewTableWriterWithWriter(w)
	mt.SetTitle("Materials")

	for _, m := range ungrouped {
		appendMaterialRows(mt, m, status, full, false)
		mt.AppendSeparator()
	}

	for _, g := range groupOrder {
		members := groupedBy[g]
		slices.SortFunc(members, byName)

		// A choke group is satisfied as soon as one of its members is set.
		satisfied := false
		for _, m := range members {
			if m.Set {
				satisfied = true
				break
			}
		}

		mt.AppendRow(table.Row{"Group", g})
		mt.AppendRow(table.Row{"Rule", fmt.Sprintf("at least one of %d required", len(members))})
		mt.AppendRow(table.Row{"Satisfied", hBool(satisfied)})
		mt.AppendSeparator()

		for _, m := range members {
			appendMaterialRows(mt, m, status, full, true)
			mt.AppendSeparator()
		}
	}

	mt.Render()

	return nil
}

// appendMaterialRows renders a single material as a block of rows. When the
// material belongs to a choke group, its name is indented under the group
// header and the per-material "Required" row is omitted (the group header
// carries the "at least one of" requirement instead).
func appendMaterialRows(mt table.Writer, m action.AttestationStatusMaterial, status *action.AttestationStatusResult, full, grouped bool) {
	name := m.Name
	if grouped {
		name = "↳ " + name
	}
	mt.AppendRow(table.Row{"Name", name})
	mt.AppendRow(table.Row{"Type", m.Type})
	mt.AppendRow(table.Row{"Set", hBool(m.Set)})
	if !grouped {
		mt.AppendRow(table.Row{"Required", hBool(m.Required)})
	}
	if m.IsOutput {
		mt.AppendRow(table.Row{"Is output", "Yes"})
	}
	if m.SkipUpload {
		mt.AppendRow(table.Row{"Skip upload", "Yes"})
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
				value = NotSet
			}

			mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	evs := status.PolicyEvaluations[m.Name]
	if len(evs) > 0 {
		mt.AppendRow(table.Row{"Policies", "------"})
		policiesTable(evs, mt, flagDebug)
	}
}

func hBool(b bool) string {
	if b {
		return "Yes"
	}

	return "No"
}

// Version information to be shown during the attestation process
// both during the process and at the end
func versionStringAttestation(p *action.ProjectVersion, isPushed bool) string {
	if p == nil || p.Version == "" {
		return ""
	}

	if isPushed {
		return versionStringAttFinal(p)
	}

	return versionStringAttTransient(p)
}

// Transient state
// It's a prerelease that will be released
// It's an already released version
// It's a pre-release that will not be released

func versionStringAttTransient(p *action.ProjectVersion) string {
	if p == nil {
		return ""
	}

	if p.Prerelease && p.MarkAsReleased {
		return fmt.Sprintf("%s (will be released)", p.Version)
	}

	if !p.Prerelease {
		return fmt.Sprintf("%s (already released)", p.Version)
	}

	return fmt.Sprintf("%s (prerelease)", p.Version)
}

// Final state
// The pre-release is still a pre-release
// The pre-release is released
func versionStringAttFinal(p *action.ProjectVersion) string {
	if p == nil {
		return ""
	}

	if p.Prerelease && !p.MarkAsReleased {
		return fmt.Sprintf("%s (prerelease)", p.Version)
	}

	return p.Version
}

// ansiPattern matches ANSI escape codes.
// Credits to: https://github.com/acarl005/stripansi
var ansiPattern = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

// removeAnsiCharactersFromBytes removes ANSI escape codes from bytes slices.
func removeAnsiCharactersFromBytes(input []byte) []byte {
	return ansiPattern.ReplaceAll(input, nil)
}
