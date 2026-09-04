//
// Copyright 2025-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newAttestationVerifyCmd() *cobra.Command {
	var fileOrURL string
	cmd := &cobra.Command{
		Use:                   "verify file-or-url",
		Short:                 "verify an attestation",
		Long:                  "Verify an attestation by validating its validation material against the configured trusted root",
		DisableFlagsInUseLine: true,
		Example: `  # verify local attestation
  chainloop attestation verify --bundle attestation.json

  # verify an attestation stored in an https endpoint
  chainloop attestation verify -b https://myrepository/attestation.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := action.NewAttestationVerifyAction(ActionOpts).Run(cmd.Context(), fileOrURL); err != nil {
				return fmt.Errorf("verifying attestation: %w", err)
			}

			ActionOpts.Logger.Info().Msg("attestation verified successfully")

			return nil
		},
	}

	cmd.Flags().StringVarP(&fileOrURL, "bundle", "b", "", "bundle path or URL")
	cobra.CheckErr(cmd.MarkFlagRequired("bundle"))

	return cmd
}
