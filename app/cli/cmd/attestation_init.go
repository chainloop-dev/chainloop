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
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
)

func newAttestationInitCmd() *cobra.Command {
	var (
		contractRevision  int
		attestationDryRun bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "start attestation crafting process",
		Annotations: map[string]string{
			useWorkflowRobotAccount: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := action.NewAttestationInit(
				&action.AttestationInitOpts{
					ActionsOpts: actionOpts,
					DryRun:      attestationDryRun,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to initialize attestation: %w", err)
			}

			// Initialize it
			attestationID, err := a.Run(cmd.Context(), contractRevision)
			if err != nil {
				if errors.Is(err, action.ErrAttestationAlreadyExist) {
					return err
				} else if errors.As(err, &action.ErrRunnerContextNotFound{}) {
					err = fmt.Errorf("%w. Use --dry-run flag if development", err)
				}

				return newGracefulError(err)
			}

			logger.Info().Msg("Attestation initialized! now you can check its status or add materials to it")

			// Show the status information
			statusAction, err := action.NewAttestationStatus(&action.AttestationStatusOpts{ActionsOpts: actionOpts})
			if err != nil {
				return newGracefulError(err)
			}

			res, err := statusAction.Run(cmd.Context(), attestationID)
			if err != nil {
				return newGracefulError(err)
			}

			return encodeOutput(res, attestationStatusTableOutput)
		},
	}

	cmd.Flags().BoolVar(&attestationDryRun, "dry-run", false, "do not record attestation in the control plane, useful for development")
	cmd.Flags().IntVar(&contractRevision, "contract-revision", 0, "revision of the contract to retrieve, \"latest\" by default")

	return cmd
}
