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
	"errors"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"
	"google.golang.org/grpc"
)

type AttestationAddOpts struct {
	*ActionsOpts
	ArtifactsCASConn   *grpc.ClientConn
	CASURI             string
	ConnectionInsecure bool
}

type AttestationAdd struct {
	*ActionsOpts
	c                  *crafter.Crafter
	casURI             string
	connectionInsecure bool
}

func NewAttestationAdd(cfg *AttestationAddOpts) *AttestationAdd {
	return &AttestationAdd{
		ActionsOpts: cfg.ActionsOpts,
		c: crafter.NewCrafter(
			crafter.WithLogger(&cfg.Logger),
		),
		casURI:             cfg.CASURI,
		connectionInsecure: cfg.ConnectionInsecure,
	}
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(k, v string, annotations map[string]string) error {
	if initialized := action.c.AlreadyInitialized(); !initialized {
		return ErrAttestationNotInitialized
	}

	if err := action.c.LoadCraftingState(); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return err
	}

	// Get upload creds and CASbackend for the current attestation and set up CAS client
	client := pb.NewAttestationServiceClient(action.CPConnection)
	creds, err := client.GetUploadCreds(context.Background(),
		&pb.AttestationServiceGetUploadCredsRequest{
			WorkflowRunId: action.c.CraftingState.GetAttestation().GetWorkflow().GetWorkflowRunId(),
		},
	)
	if err != nil {
		return err
	}

	b := creds.GetResult().GetBackend()
	if b == nil {
		return fmt.Errorf("no backend found in upload creds")
	}

	// Define CASbackend information based on the API response
	casBackend := &casclient.CASBackend{
		Name:    b.Provider,
		MaxSize: b.GetLimits().MaxBytes,
	}

	// Some CASBackends will actually upload information to the CAS server
	// in such case we need to set up a connection
	if !b.IsInline && creds.Result.Token != "" {
		artifactCASConn, err := grpcconn.New(action.casURI, creds.Result.Token, action.connectionInsecure)
		if err != nil {
			return err
		}
		defer artifactCASConn.Close()

		casBackend.Uploader = casclient.New(artifactCASConn, casclient.WithLogger(action.Logger))
	}

	if err := action.c.AddMaterial(k, v, casBackend, annotations); err != nil {
		return fmt.Errorf("adding material: %w", err)
	}

	return nil
}
