//
// Copyright 2024-2026 The Chainloop Authors.
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
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

//nolint:gosec
const artifactCASTokenEnvVarName = "CHAINLOOP_ARTIFACT_CAS_TOKEN"

func newArtifactDownloadCmd() *cobra.Command {
	var digest, downloadPath, outputFile, casTokenFlag string
	var artifactCASConn *grpc.ClientConn
	var actionOpts *action.ActionsOpts

	cmd := &cobra.Command{
		Use:   "download",
		Short: "download an artifact",
		// Root does not build the control plane connection for this command: PreRunE builds it
		// only when no CAS credential is set. Telemetry reports the command in both cases.
		Annotations: map[string]string{skipActionOptsInit: trueString, reportTelemetryAnyway: trueString},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateFlags(downloadPath, outputFile); err != nil {
				return err
			}

			casToken, err := artifactCASToken(casTokenFlag)
			if err != nil {
				return err
			}

			// A CAS credential replaces the API token and the control plane
			var cpConn *grpc.ClientConn
			if casToken == "" {
				if err := ensureActionOpts(cmd); err != nil {
					return err
				}
				actionOpts, cpConn = ActionOpts, ActionOpts.CPConnection
			} else {
				logger.Debug().Msg("using the given CAS credential, the control plane is not contacted")
				actionOpts = newActionOpts(logger, nil, "")
			}

			artifactCASConn, err = wrappedArtifactConn(cpConn,
				pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER, digest, casToken)
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := &action.ArtifactDownloadOpts{
				ActionsOpts:      actionOpts,
				ArtifactsCASConn: artifactCASConn,
				Stdout:           cmd.OutOrStdout(),
			}

			return action.NewArtifactDownload(opts).Run(downloadPath, outputFile, digest)
		},
		PostRunE: func(_ *cobra.Command, _ []string) error {
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
	cmd.Flags().StringVar(&casTokenFlag, "artifact-cas-token", "", fmt.Sprintf("CAS download credential for --digest, used instead of the API token and the control plane. NOTE: Prefer the env variable %s", artifactCASTokenEnvVarName))

	return cmd
}

// artifactCASToken returns the CAS credential from the flag or, when the flag is empty,
// from the environment. The CAS verifies the credential; this only rejects a non-JWT value.
func artifactCASToken(flagValue string) (string, error) {
	token := flagValue
	if token == "" {
		token = os.Getenv(artifactCASTokenEnvVarName)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil
	}

	if parts := strings.Split(token, "."); len(parts) != 3 || slices.Contains(parts, "") {
		return "", errors.New("the CAS credential is not a JWT")
	}

	return token, nil
}

// validateFlags checks if the flags are valid
func validateFlags(downloadPath, outputFile string) error {
	if downloadPath != "" && outputFile != "" {
		return errors.New("cannot specify both --path and --output flags at the same time")
	}

	return nil
}
