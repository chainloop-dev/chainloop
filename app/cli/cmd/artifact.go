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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func newArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Download or upload Artifacts to the CAS",
	}

	cmd.AddCommand(newArtifactUploadCmd(), newArtifactDownloadCmd())
	return cmd
}

func wrappedArtifactConn(cpConn *grpc.ClientConn, role pb.CASCredentialsServiceGetRequest_Role) (*grpc.ClientConn, error) {
	// Retrieve temporary credentials for uploading
	client := pb.NewCASCredentialsServiceClient(cpConn)
	resp, err := client.Get(context.Background(), &pb.CASCredentialsServiceGetRequest{
		Role: role,
	})
	if err != nil {
		return nil, err
	}

	return newGRPCConnection(viper.GetString(confOptions.CASAPI.viperKey), resp.Result.Token, flagInsecure, logger)
}
