//
// Copyright 2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/spf13/cobra"
)

const traceExportLong = `Write the recorded AI coding sessions of this repository to a local directory,
instead of pushing a signed attestation.

It runs the same assembly a push does: it reads each spec source, redacts
secrets, replaces copies with pointers, and stores each spec, skill, and image
as its own material. Every material is written under its content digest, next to
a manifest that lists them, so the output equals what a push would upload apart
from the signature.

It makes no call to the control plane and needs no credentials or network. Pair
it with "chainloop trace init --local": that sets the repository up to record
sessions without a backend, and this writes their evidence to disk. --no-redact
turns off secret redaction for a trusted local run, so the output can then hold
secrets.`

// newTraceExportCmd creates the `trace export` subcommand.
func newTraceExportCmd() *cobra.Command {
	var noRedact bool

	cmd := &cobra.Command{
		Use:   "export <dir>",
		Short: "Write the recorded AI coding session evidence to a local directory",
		Long:  traceExportLong,
		Args:  cobra.ExactArgs(1),
		// The export talks to no control plane, so it needs no authenticated
		// action options.
		Annotations: map[string]string{
			skipActionOptsInit: trueString,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := action.RunTraceExport(cmd.Context(), logger, action.RunTraceExportOpts{
				OutDir:   args[0],
				NoRedact: noRedact,
				Mode:     aicodingsession.ModeCoding,
			})
			if err != nil {
				return err
			}

			if dir == "" {
				logger.Info().Msg("no AI coding session was recorded, so nothing was exported")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noRedact, "no-redact", false, "disable secret redaction; for a trusted local run only, as the output can then hold secrets")

	return cmd
}
