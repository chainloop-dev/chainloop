//
// Copyright 2023-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func newArtifactUploadCmd() *cobra.Command {
	var filePath, digest string
	var file *os.File
	var artifactCASConn *grpc.ClientConn

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "upload an artifact",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			// The file stays open until the upload, so the uploaded content is the hashed content
			file, err = os.Open(filePath)
			if err != nil {
				return fmt.Errorf("can't open file to upload: %w", err)
			}

			digest, err = casclient.Digest(file)
			if err != nil {
				return err
			}

			// Retrieve temporary credentials for uploading, scoped to the file digest
			artifactCASConn, err = wrappedArtifactConn(ActionOpts.CPConnection, pb.CASCredentialsServiceGetRequest_ROLE_UPLOADER, digest)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &action.ArtifactUploadOpts{
				ActionsOpts:      ActionOpts,
				ArtifactsCASConn: artifactCASConn,
			}

			_, err := action.NewArtifactUpload(opts).Run(file, digest)
			return err
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			var errs []error
			if file != nil {
				errs = append(errs, file.Close())
			}

			if artifactCASConn != nil {
				errs = append(errs, artifactCASConn.Close())
			}

			return errors.Join(errs...)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "path to file to upload")
	err := cmd.MarkFlagRequired("file")
	cobra.CheckErr(err)

	return cmd
}
