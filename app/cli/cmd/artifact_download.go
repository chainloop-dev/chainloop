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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func newArtifactDownloadCmd() *cobra.Command {
	var digest, downloadPath, outputFile string
	var artifactCASConn *grpc.ClientConn

	cmd := &cobra.Command{
		Use:   "download",
		Short: "download an artifact",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if err := validateFlags(downloadPath, outputFile); err != nil {
				return err
			}

			// Retrieve temporary credentials for uploading
			artifactCASConn, err = wrappedArtifactConn(actionOpts.CPConnection,
				pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER, digest)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.ArtifactDownloadOpts{
				ActionsOpts:      actionOpts,
				ArtifactsCASConn: artifactCASConn,
				Stdout:           cmd.OutOrStdout(),
			}

			return action.NewArtifactDownload(opts).Run(downloadPath, outputFile, digest)
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			if artifactCASConn != nil {
				return artifactCASConn.Close()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&digest, "digest", "d", "", "digest of the file to download")
	err := cmd.MarkFlagRequired("digest")
	cobra.CheckErr(err)
	cmd.Flags().StringVar(&downloadPath, "path", "", "download path, default to current directory")
	cmd.Flags().StringVar(&outputFile, "output", "", "The `file` to write a single asset to (use \"-\" to write to standard output")

	return cmd
}

// validateFlags checks if the flags are valid
func validateFlags(downloadPath, outputFile string) error {
	if downloadPath != "" && outputFile != "" {
		return errors.New("cannot specify both --path and --output flags at the same time")
	}

	return nil
}
