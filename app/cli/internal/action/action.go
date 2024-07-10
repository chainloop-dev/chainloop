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

package action

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager/filesystem"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager/remote"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type ActionsOpts struct {
	CPConnection              *grpc.ClientConn
	Logger                    zerolog.Logger
	UseAttestationRemoteState bool
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}

// load a crafter with either local or remote state
func newCrafter(enableRemoteState bool, conn *grpc.ClientConn, opts ...crafter.NewOpt) (*crafter.Crafter, error) {
	var stateManager crafter.StateManager
	var err error

	switch enableRemoteState {
	case true:
		stateManager, err = remote.New(pb.NewAttestationStateServiceClient(conn))
	case false:
		stateManager, err = filesystem.New(filepath.Join(os.TempDir(), "chainloop-attestation.tmp.json"))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	return crafter.NewCrafter(stateManager, opts...)
}

// creates a connection to a CAS backend
func getCasBackend(ctx context.Context, state *v1.CraftingState, opts *ActionsOpts, casCAPath, casURI string, insecure bool) (*casclient.CASBackend, func() error, error) {
	// Default to inline CASBackend and override if we are not in dry-run mode
	var closefunc func() error
	backend := &casclient.CASBackend{
		Name: "not-set",
	}

	// Define CASbackend information based on the API response
	if !state.GetDryRun() {
		// Get upload creds and CASbackend for the current attestation and set up CAS client
		client := pb.NewAttestationServiceClient(opts.CPConnection)
		creds, err := client.GetUploadCreds(ctx,
			&pb.AttestationServiceGetUploadCredsRequest{
				WorkflowRunId: state.GetAttestation().GetWorkflow().GetWorkflowRunId(),
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get upload creds: %w", err)
		}
		b := creds.GetResult().GetBackend()
		if b == nil {
			return nil, nil, fmt.Errorf("no backend found in upload creds")
		}
		backend.Name = b.Provider
		backend.MaxSize = b.GetLimits().MaxBytes

		// Some CASBackends will actually upload information to the CAS server
		// in such case we need to set up a connection
		if !b.IsInline && creds.Result.Token != "" {
			var grpcopts = []grpcconn.Option{
				grpcconn.WithInsecure(insecure),
			}

			if casCAPath != "" {
				grpcopts = append(grpcopts, grpcconn.WithCAFile(casCAPath))
			}

			artifactCASConn, err := grpcconn.New(casURI, creds.Result.Token, grpcopts...)
			closefunc = artifactCASConn.Close

			if err != nil {
				return nil, nil, fmt.Errorf("failed to create CAS client: %w", err)
			}

			cascli := casclient.New(artifactCASConn, casclient.WithLogger(opts.Logger))
			backend.Uploader = cascli
			backend.Downloader = cascli
		}
	}

	return backend, closefunc, nil
}
