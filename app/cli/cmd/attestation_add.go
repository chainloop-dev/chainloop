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
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
)

func newAttestationAddCmd() *cobra.Command {
	var name, value string
	var artifactCASConn *grpc.ClientConn
	var annotationsFlag []string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "add a material to the attestation",
		Annotations: map[string]string{
			useWorkflowRobotAccount: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := action.NewAttestationAdd(
				&action.AttestationAddOpts{
					ActionsOpts:        actionOpts,
					CASURI:             viper.GetString(confOptions.CASAPI.viperKey),
					ConnectionInsecure: flagInsecure,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to load action: %w", err)
			}

			// Extract annotations
			annotations, err := extractAnnotations(annotationsFlag)
			if err != nil {
				return err
			}

			if err := a.Run(cmd.Context(), attestationID, name, value, annotations); err != nil {
				if errors.Is(err, action.ErrAttestationNotInitialized) {
					return err
				}

				return newGracefulError(err)
			}

			logger.Info().Msg("material added to attestation")

			return nil
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			if artifactCASConn != nil {
				return artifactCASConn.Close()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "name of the material to be recorded")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&value, "value", "", "value to be recorded")
	err = cmd.MarkFlagRequired("value")
	cobra.CheckErr(err)
	cmd.Flags().StringSliceVar(&annotationsFlag, "annotation", nil, "additional annotation in the format of key=value")
	flagAttestationID(cmd)

	return cmd
}
