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
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func newArtifactUploadCmd() *cobra.Command {
	var filePath string
	var artifactCASConn *grpc.ClientConn

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "upload an artifact",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			// Retrieve temporary credentials for uploading
			artifactCASConn, err = wrappedArtifactConn(actionOpts.CPConnection, pb.CASCredentialsServiceGetRequest_ROLE_UPLOADER, "")
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.ArtifactUploadOpts{
				ActionsOpts:      actionOpts,
				ArtifactsCASConn: artifactCASConn,
			}

			_, err := action.NewArtifactUpload(opts).Run(filePath)
			return err
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			if artifactCASConn != nil {
				return artifactCASConn.Close()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "path to file to upload")
	err := cmd.MarkFlagRequired("file")
	cobra.CheckErr(err)

	return cmd
}
