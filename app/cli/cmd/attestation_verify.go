//
// Copyright 22025 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newAttestationVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "verify file-or-url",
		Short:                 "verify an attestation",
		Long:                  "Verify an attestation by validating its validation material against the configured trusted root",
		DisableFlagsInUseLine: true,
		Example: `  # verify local attestation
  chainloop attestation verify attestation.json

  # verify an attestation stored in an https endpoint
  chainloop attestation verify https://myrepository/attestation.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewAttestationVerifyAction(actionOpts).Run(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("verifying attestation: %w", err)
			}
			if res {
				actionOpts.Logger.Info().Msg("attestation verified successfully")
			} else {
				actionOpts.Logger.Warn().Msg("attestation couldn't be verified")
			}

			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	return cmd
}
