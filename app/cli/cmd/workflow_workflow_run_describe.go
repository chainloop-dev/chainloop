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
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
)

const formatStatement = "statement"
const formatAttestation = "attestation"

func newWorkflowWorkflowRunDescribeCmd() *cobra.Command {
	var runID, attestationDigest, publicKey string
	var verifyAttestation bool
	// TODO: Replace by retrieving key from rekor
	const signingKeyEnvVarName = "CHAINLOOP_SIGNING_PUBLIC_KEY"

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "View a Workflow Run",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if verifyAttestation && publicKey == "" {
				return errors.New("a public key needs to be provided for verification")
			}

			if runID == "" && attestationDigest == "" {
				return errors.New("either a run ID or the attestation digest needs to be provided")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowRunDescribe(actionOpts).Run(context.Background(), runID, attestationDigest, verifyAttestation, publicKey)
			if err != nil {
				return err
			}

			return encodeAttestationOutput(res)
		},
	}

	cmd.Flags().StringVar(&runID, "id", "", "workflow Run ID")
	cmd.Flags().StringVar(&attestationDigest, "digest", "", "content digest of the attestation")

	cmd.Flags().BoolVar(&verifyAttestation, "verify", false, "verify the attestation")
	cmd.Flags().StringVar(&publicKey, "key", "", fmt.Sprintf("public key used to verify the attestation. Note: You can also use env variable %s", signingKeyEnvVarName))

	if publicKey == "" {
		publicKey = os.Getenv(signingKeyEnvVarName)
	}

	// Override default output flag
	cmd.InheritedFlags().StringVarP(&flagOutputFormat, "output", "o", "table", "output format, valid options are table, json, attestation or statement")

	return cmd
}

func workflowRunDescribeTableOutput(run *action.WorkflowRunItemFull) error {
	// General info table
	wf := run.Workflow

	gt := newTableWriter()
	gt.SetTitle("Workflow")
	gt.AppendRow(table.Row{"ID", wf.ID})
	gt.AppendRow(table.Row{"Name", wf.Name})
	gt.AppendRow(table.Row{"Team", wf.Team})
	gt.AppendRow(table.Row{"Project", wf.Project})
	gt.AppendSeparator()

	wr := run.WorkflowRun
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
	gt.Render()

	predicateV1Table(att)
	logger.Info().Msg("you can use the flag \"--output statement\" to see the full in-toto statement")
	return nil
}

func predicateV1Table(att *action.WorkflowRunAttestationItem) {
	// Materials
	materials := att.Materials
	if len(materials) > 0 {
		mt := newTableWriter()
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
				mt.AppendRow(table.Row{"Value", wrap.String(v, 100)})
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
			mt.AppendSeparator()
		}

		mt.Render()
	}

	envVars := att.EnvVars
	if len(envVars) > 0 {
		mt := newTableWriter()
		mt.SetTitle("Environment Variables")

		header := table.Row{"Name", "Value"}
		mt.AppendHeader(header)
		for _, e := range envVars {
			mt.AppendRow(table.Row{e.Name, e.Value})
		}
		mt.Render()
	}
}

func encodeAttestationOutput(run *action.WorkflowRunItemFull) error {
	// Try to encode as a table or json
	err := encodeOutput(run, workflowRunDescribeTableOutput)
	// It was correctly encoded, we are done
	if err == nil {
		return nil
	}

	// It could not be encoded but for a reason that's not because it was a custom format
	if !errors.Is(err, ErrOutputFormatNotImplemented) {
		return err
	}

	// Try to encode the output using some additional custom formats
	if run.Attestation == nil {
		logger.Info().Msg("This run doesn't have an attestation, noop")
		return nil
	}

	switch flagOutputFormat {
	case formatStatement:
		return encodeJSON(run.Attestation.Statement())
	case formatAttestation:
		return encodeJSON(run.Attestation.Envelope)
	default:
		return ErrOutputFormatNotImplemented
	}
}
