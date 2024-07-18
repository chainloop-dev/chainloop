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
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

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
			useAPIToken: "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if workflowName == "" {
				return errors.New("workflow name is required, set it via --name flag")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := action.NewAttestationInit(
				&action.AttestationInitOpts{
					ActionsOpts:    actionOpts,
					DryRun:         attestationDryRun,
					Force:          force,
					UseRemoteState: useAttestationRemoteState,
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
			statusAction, err := action.NewAttestationStatus(&action.AttestationStatusOpts{ActionsOpts: actionOpts, UseAttestationRemoteState: useAttestationRemoteState})
			if err != nil {
				return newGracefulError(err)
			}

			res, err := statusAction.Run(cmd.Context(), attestationID)
			if err != nil {
				return newGracefulError(err)
			}

			if res.DryRun {
				logger.Info().Msg("The attestation is being crafted in dry-run mode. It will not get stored once rendered")
			}

			return encodeOutput(res, simpleStatusTable)
		},
	}

	// This option is only useful for local-based attestation states
	cmd.Flags().BoolVarP(&force, "replace", "f", false, "replace any existing in-progress attestation")
	cmd.Flags().BoolVar(&attestationDryRun, "dry-run", false, "do not record attestation in the control plane, useful for development")
	cmd.Flags().IntVar(&contractRevision, "contract-revision", 0, "revision of the contract to retrieve, \"latest\" by default")
	cmd.Flags().BoolVar(&useAttestationRemoteState, "remote-state", false, "Store the attestation state remotely")

	// workflow-name has been replaced by --name flag
	cmd.Flags().StringVar(&workflowName, "workflow-name", "", "name of the workflow to run the attestation")
	cobra.CheckErr(cmd.Flags().MarkHidden("workflow-name"))
	cmd.Flags().StringVar(&workflowName, "name", "", "name of the workflow to run the attestation")

	return cmd
}
