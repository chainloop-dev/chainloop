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

	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newAttestationResetCmd() *cobra.Command {
	var trigger, reason string
	triggerFailed, triggerCanceled := action.AttestationResetTriggerFailed, action.AttestationResetTriggerCancelled

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "mark current attestation process as canceled or failed",
		Annotations: map[string]string{
			useWorkflowRobotAccount: "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if trigger != triggerFailed && trigger != triggerCanceled {
				return errors.New("--trigger value is invalid")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := action.NewAttestationReset(actionOpts)

			if err := a.Run(trigger, reason); err != nil {
				return newGracefulError(err)
			}

			logger.Info().Msg("Attestation canceled")

			return nil
		},
	}

	cmd.Flags().StringVar(&trigger, "trigger", triggerFailed, fmt.Sprintf("trigger for the reset, valid options are %q and %q", triggerFailed, triggerCanceled))
	cmd.Flags().StringVar(&reason, "reason", "", "reset reason")

	return cmd
}
