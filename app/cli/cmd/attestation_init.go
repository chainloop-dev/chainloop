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
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

const workflowNameEnvVarName = "CHAINLOOP_WORKFLOW_NAME"

func newAttestationInitCmd() *cobra.Command {
	var (
		force             bool
		contractRevision  int
		attestationDryRun bool
		workflowName      string
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
					Force:       force,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to initialize attestation: %w", err)
			}

			// Initialize it
			attestationID, err := a.Run(cmd.Context(), contractRevision, workflowName)
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

			return encodeOutput(res, simpleStatusTable)
		},
	}

	// This option is only useful for local-based attestation states
	cmd.Flags().BoolVarP(&force, "replace", "f", false, "replace any existing in-progress attestation")
	cmd.Flags().BoolVar(&attestationDryRun, "dry-run", false, "do not record attestation in the control plane, useful for development")
	cmd.Flags().IntVar(&contractRevision, "contract-revision", 0, "revision of the contract to retrieve, \"latest\" by default")
	cmd.Flags().StringVar(&workflowName, "workflow-name", "", "name of the workflow to run the attestation. This is ignored when authentication is based on Robot Account")
	if workflowName == "" {
		workflowName = os.Getenv(workflowNameEnvVarName)
	}

	return cmd
}
