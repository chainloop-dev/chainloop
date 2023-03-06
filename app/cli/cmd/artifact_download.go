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
	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	pb "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func newArtifactDownloadCmd() *cobra.Command {
	var digest, downloadPath string
	var artifactCASConn *grpc.ClientConn

	cmd := &cobra.Command{
		Use:   "download",
		Short: "download an artifact",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			// Retrieve temporary credentials for uploading
			artifactCASConn, err = wrappedArtifactConn(actionOpts.CPConnecction,
				pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.ArtifactDownloadOpts{
				ActionsOpts:      actionOpts,
				ArtifactsCASConn: artifactCASConn,
			}

			return action.NewArtifactDownload(opts).Run(downloadPath, digest)
		},
	}

	cmd.Flags().StringVarP(&digest, "digest", "d", "", "digest of the file to download")
	err := cmd.MarkFlagRequired("digest")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&downloadPath, "path", "", "download path, default to current directory")

	return cmd
}
