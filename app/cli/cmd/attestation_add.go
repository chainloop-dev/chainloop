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
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	pb "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
)

func newAttestationAddCmd() *cobra.Command {
	var name, value string
	var artifactCASConn *grpc.ClientConn

	cmd := &cobra.Command{
		Use:   "add",
		Short: "add a material to the attestation",
		Annotations: map[string]string{
			useWorkflowRobotAccount: "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			// Retrieve temporary credentials for uploading
			// TODO: only do it for artifact uploads
			client := pb.NewAttestationServiceClient(actionOpts.CPConnecction)
			resp, err := client.GetUploadCreds(context.Background(), &pb.AttestationServiceGetUploadCredsRequest{})
			if err != nil {
				return newGracefulError(err)
			}

			artifactCASConn, err = newGRPCConnection(viper.GetString(confOptions.CASAPI.viperKey), resp.Result.Token, flagInsecure, logger)
			if err != nil {
				return newGracefulError(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := action.NewAttestationAdd(
				&action.AttestationAddOpts{ActionsOpts: actionOpts, ArtifacsCASConn: artifactCASConn},
			)

			err := a.Run(name, value)
			if err != nil {
				if errors.Is(err, action.ErrAttestationNotInitialized) {
					return err
				}

				return newGracefulError(err)
			}

			logger.Info().Msg("material added to attestation")

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "name of the material to be recorded")
	cmd.Flags().StringVar(&value, "value", "", "value to be recorded")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)
	err = cmd.MarkFlagRequired("value")
	cobra.CheckErr(err)

	return cmd
}
