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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	attv1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/muesli/reflow/wrap"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

const formatStatement = "statement"
const formatAttestation = "attestation"

// outputs the payload in PAE encoding, so that it matches the signature in the attestation,
// and it's easily verifiable by external tools
const formatPayloadPAE = "payload-pae"

func newWorkflowWorkflowRunDescribeCmd() *cobra.Command {
	var (
		runID, attestationDigest, publicKey string
		certPath, chainPath                 string
		verifyAttestation                   bool
	)

	// TODO: Replace by retrieving key from rekor
	const signingKeyEnvVarName = "CHAINLOOP_SIGNING_PUBLIC_KEY"

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "View a Workflow Run",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if verifyAttestation && publicKey == "" && certPath == "" {
				return errors.New("a public key or certificate needs to be provided for verification")
			}

			if runID == "" && attestationDigest == "" {
				return errors.New("either a run ID or the attestation digest needs to be provided")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRunDescribe(ActionOpts).Run(context.Background(), &action.WorkflowRunDescribeOpts{
				RunID:         runID,
				Digest:        attestationDigest,
				PublicKeyRef:  publicKey,
				CertPath:      certPath,
				CertChainPath: chainPath,
				Verify:        verifyAttestation,
			})
			if err != nil {
				return err
			}

			return encodeAttestationOutput(res, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&runID, "id", "", "workflow Run ID")
	cmd.Flags().StringVarP(&attestationDigest, "digest", "d", "", "content digest of the attestation")

	cmd.Flags().BoolVar(&verifyAttestation, "verify", false, "verify the attestation")
	cmd.Flags().StringVar(&publicKey, "key", "", fmt.Sprintf("public key used to verify the attestation. Note: You can also use env variable %s", signingKeyEnvVarName))

	if publicKey == "" {
		publicKey = os.Getenv(signingKeyEnvVarName)
	}

	cmd.Flags().StringVar(&certPath, "cert", "", "public certificate in PEM format to be used to verify the attestation")
	cmd.Flags().StringVar(&chainPath, "cert-chain", "", "certificate chain (intermediates, root) in PEM format to be used to verify the attestation")

	// Override default output flag
	cmd.InheritedFlags().StringVarP(&flagOutputFormat, "output", "o", "table", "output format, valid options are table, json, attestation, statement or payload-pae")

	return cmd
}

func workflowRunDescribeTableOutput(run *action.WorkflowRunItemFull) error {
	// General info table
	wf := run.Workflow
	wr := run.WorkflowRun

	gt := output.NewTableWriter()
	gt.SetTitle("Workflow")
	gt.AppendRow(table.Row{"ID", wf.ID})
	gt.AppendRow(table.Row{"Name", wf.Name})
	gt.AppendRow(table.Row{"Team", wf.Team})
	gt.AppendRow(table.Row{"Project", wf.Project})
	gt.AppendRow(table.Row{"Version", versionString(wr.ProjectVersion)})
	gt.AppendSeparator()

	gt.AppendRow(table.Row{"Workflow Run"})
	gt.AppendSeparator()
	gt.AppendRow(table.Row{"ID", wr.ID})
	gt.AppendRow(table.Row{"Initialized At", wr.CreatedAt.Format(time.RFC822)})
	if fa := wr.FinishedAt; fa != nil {
		gt.AppendRow(table.Row{"Finished At", wr.FinishedAt.Format(time.RFC822)})
	}
	gt.AppendRow(table.Row{"State", wr.State})
	if wr.Reason != "" {
		gt.AppendRow(table.Row{"Failure Reason", wr.Reason})
	}
	gt.AppendRow(table.Row{"Policy Status", wr.PolicyStatus})
	gt.AppendRow(table.Row{"Runner Link", wr.RunURL})

	if run.WorkflowRun.FinishedAt == nil {
		gt.Render()
		logger.Info().Msg("the attestation crafting process is in progress, it has not been received yet")
		return nil
	}

	att := run.Attestation
	if att == nil {
		gt.Render()
		logger.Warn().Msg("there was an issue retrieving the attestation")
		return nil
	}

	gt.AppendSeparator()
	gt.AppendRow(table.Row{"Statement"})
	gt.AppendSeparator()
	gt.AppendRow(table.Row{"Payload Type", att.Envelope.PayloadType})
	gt.AppendRow(table.Row{"Digest", att.Digest})
	color := text.FgHiRed
	if run.Verified {
		color = text.FgHiGreen
	}
	gt.AppendRow(table.Row{"Verified", color.Sprint(run.Verified)})
	if len(att.Annotations) > 0 {
		gt.AppendRow(table.Row{"Annotations", "------"})
		for _, a := range att.Annotations {
			gt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, a.Value)})
		}
	}

	gt.AppendRow(table.Row{"Policies violation strategy", att.PolicyEvaluationStatus.Strategy})
	if att.PolicyEvaluationStatus.Blocked {
		gt.AppendRow(table.Row{"Run Blocked", att.PolicyEvaluationStatus.Blocked})
	}
	if att.PolicyEvaluationStatus.HasGatedViolations {
		gt.AppendRow(table.Row{"Run Gated", text.Colors{text.FgHiRed}.Sprint(att.PolicyEvaluationStatus.HasGatedViolations)})
	}
	if att.PolicyEvaluationStatus.Strategy == action.PolicyViolationBlockingStrategyEnforced {
		gt.AppendRow(table.Row{"Policy enforcement bypassed", att.PolicyEvaluationStatus.Bypassed})
	}

	evs := att.PolicyEvaluations[chainloop.AttPolicyEvaluation]
	if len(evs) > 0 {
		gt.AppendRow(table.Row{"Policies", "------"})
		policiesTable(evs, gt, flagDebug)
	}

	if run.Attestation.AttestationViewURL != "" {
		gt.AppendRow(table.Row{"Attestation View URL", run.Attestation.AttestationViewURL})
	}

	gt.Render()

	predicateV1Table(att)
	logger.Info().Msg("you can use the flag \"--output statement\" to see the full in-toto statement")
	return nil
}

func predicateV1Table(att *action.WorkflowRunAttestationItem) {
	// Materials
	materials := att.Materials
	if len(materials) > 0 {
		mt := output.NewTableWriter()
		mt.SetTitle("Materials")

		for _, m := range materials {
			mt.AppendRow(table.Row{"Name", m.Name})
			mt.AppendRow(table.Row{"Type", m.Type})
			if m.Filename != "" {
				mt.AppendRow(table.Row{"Filename", m.Filename})
			}

			// We do not want to show the value if it is a file
			if !m.EmbeddedInline && m.UploadedToCAS || m.Type == "CONTAINER_IMAGE" {
				v := m.Value
				if m.Tag != "" {
					v = fmt.Sprintf("%s:%s", v, m.Tag)
				}
				if v != "" {
					mt.AppendRow(table.Row{"Value", wrap.String(v, 100)})
				}
			}

			if m.Hash != "" {
				mt.AppendRow(table.Row{"Digest", m.Hash})
			}

			if len(m.Annotations) > 0 {
				mt.AppendRow(table.Row{"Annotations", "------"})
				for _, a := range m.Annotations {
					mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, a.Value)})
				}
			}
			evs := att.PolicyEvaluations[m.Name]
			if len(evs) > 0 {
				mt.AppendRow(table.Row{"Policies", "------"})
				policiesTable(evs, mt, flagDebug)
			}
			mt.AppendSeparator()
		}

		mt.Render()
	}

	envVars := att.EnvVars
	if len(envVars) > 0 {
		mt := output.NewTableWriter()
		mt.SetTitle("Environment Variables")

		header := table.Row{"Name", "Value"}
		mt.AppendHeader(header)
		for _, e := range envVars {
			mt.AppendRow(table.Row{e.Name, e.Value})
		}
		mt.Render()
	}
}

func policiesTable(evs []*action.PolicyEvaluation, mt table.Writer, debugMode bool) {
	for _, ev := range evs {
		msg := ""

		// Partition: active violations count toward the gate; suppressed
		// entries are kept in the CAS bundle for audit and shown separately
		// so operators can see policy decisions without losing context.
		var active []string
		var suppressed []*action.PolicyViolation
		for _, v := range ev.Violations {
			if v.Suppress {
				suppressed = append(suppressed, v)
				continue
			}
			active = append(active, v.Message)
		}

		switch {
		case ev.Skipped:
			switch {
			case len(ev.SkipReasons) == 1:
				msg = text.Colors{text.FgHiYellow}.Sprintf("skipped - %s", ev.SkipReasons[0])
			case debugMode:
				msg = text.Colors{text.FgHiYellow}.Sprintf("skipped - multiple reasons:\n  - %s",
					strings.Join(ev.SkipReasons, "\n  - "))
			default:
				msg = text.Colors{text.FgHiYellow}.Sprint("the policy was skipped in all execution paths")
			}
		case len(active) == 0:
			msg = text.Colors{text.FgHiGreen}.Sprint("Ok")
		default:
			color := text.Colors{text.FgHiRed}
			var prefix = ""
			// For multiple violations, we want to indent the list
			if len(active) > 1 {
				prefix = "\n  - "
			}

			// Color the violations text before joining
			for i, v := range active {
				active[i] = color.Sprint(v)
			}

			msg = prefix + strings.Join(active, prefix)
		}

		if s := renderSuppressed(suppressed); s != "" {
			msg = msg + "\n" + s
		}

		name := ev.Name
		if ev.Gate {
			name = fmt.Sprintf("%s (gate)", ev.Name)
		}
		mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", name, msg)})
	}
}

// renderSuppressed formats a "Suppressed (N)" sub-section listing entries the
// policy excluded from the gate. Each line shows the violation message plus,
// when a structured finding with an assessment is available, the
// precedence-resolved status and scope (e.g. "NOT_AFFECTED, PROJECT") so
// operators can audit suppression decisions without downloading the bundle.
func renderSuppressed(suppressed []*action.PolicyViolation) string {
	if len(suppressed) == 0 {
		return ""
	}
	dim := text.Colors{text.FgHiYellow}
	lines := make([]string, 0, len(suppressed))
	for _, v := range suppressed {
		line := v.Message
		if a := suppressedAssessment(v); a != "" {
			line = fmt.Sprintf("%s — %s", line, a)
		}
		lines = append(lines, "  - "+line)
	}
	header := dim.Sprintf("Suppressed (%d):", len(suppressed))
	return header + "\n" + dim.Sprint(strings.Join(lines, "\n"))
}

// suppressedAssessment extracts the effective assessment status and scope
// from whichever structured finding is attached to the violation, if any.
// Returns the empty string when no assessment is available (unstructured
// policy, or finding without an assessment annotation).
func suppressedAssessment(v *action.PolicyViolation) string {
	switch {
	case v.Vulnerability != nil && v.Vulnerability.Assessment != nil:
		return prettyAssessment(v.Vulnerability.Assessment.GetEffectiveStatus(), assessmentScopes(v.Vulnerability.Assessment.GetAssessments()))
	case v.Sast != nil && v.Sast.Assessment != nil:
		return prettyAssessment(v.Sast.Assessment.GetEffectiveStatus(), assessmentScopes(v.Sast.Assessment.GetAssessments()))
	case v.LicenseViolation != nil && v.LicenseViolation.Assessment != nil:
		return prettyAssessment(v.LicenseViolation.Assessment.GetEffectiveStatus(), assessmentScopes(v.LicenseViolation.Assessment.GetAssessments()))
	}
	return ""
}

func prettyAssessment(status string, scopes []string) string {
	s := strings.TrimPrefix(status, "ASSESSMENT_STATUS_")
	if s == "" {
		return ""
	}
	if len(scopes) == 0 {
		return s
	}
	return fmt.Sprintf("%s, %s scope", s, strings.Join(scopes, "/"))
}

func assessmentScopes(in []*attv1.PolicyAssessment) []string {
	scopes := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, a := range in {
		scope := strings.TrimPrefix(a.GetScope(), "ASSESSMENT_SCOPE_")
		if scope == "" {
			continue
		}
		if _, dup := seen[scope]; dup {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	return scopes
}

func encodeAttestationOutput(run *action.WorkflowRunItemFull, writer io.Writer) error {
	// Try to encode as a table or json
	err := output.EncodeOutput(flagOutputFormat, run, workflowRunDescribeTableOutput)
	// It was correctly encoded, we are done
	if err == nil {
		return nil
	}

	// It could not be encoded but for a reason that's not because it was a custom format
	if !errors.Is(err, output.ErrOutputFormatNotImplemented) {
		return err
	}

	// Try to encode the output using some additional custom formats
	if run.Attestation == nil {
		logger.Info().Msg("This run doesn't have an attestation, noop")
		return nil
	}

	switch flagOutputFormat {
	case formatStatement:
		return output.EncodeJSON(run.Attestation.Statement())
	case formatAttestation:
		if run.Attestation.Bundle != nil {
			var bundle protobundle.Bundle
			err = protojson.Unmarshal(run.Attestation.Bundle, &bundle)
			if err != nil {
				return fmt.Errorf("unmarshaling attestation: %w", err)
			}
			return output.EncodeProtoJSON(&bundle)
		} else {
			return output.EncodeJSON(run.Attestation.Envelope)
		}
	case formatPayloadPAE:
		return encodePAE(run, writer)
	default:
		return output.ErrOutputFormatNotImplemented
	}
}

func encodePAE(run *action.WorkflowRunItemFull, writer io.Writer) error {
	payload, err := run.Attestation.Envelope.DecodeB64Payload()
	if err != nil {
		return fmt.Errorf("could not decode attestation payload: %w", err)
	}
	_, err = fmt.Fprint(writer, string(dsse.PAE(run.Attestation.Envelope.PayloadType, payload)))
	return err
}
