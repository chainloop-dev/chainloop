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
	"encoding/json"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/grpc"
)

type AttestationPushOpts struct {
	*ActionsOpts
	KeyPath, CLIversion, CLIDigest string
}

type AttestationPush struct {
	*ActionsOpts
	c                              *crafter.Crafter
	keyPath, cliVersion, cliDigest string
}

func NewAttestationPush(cfg *AttestationPushOpts) *AttestationPush {
	return &AttestationPush{
		ActionsOpts: cfg.ActionsOpts,
		c:           crafter.NewCrafter(crafter.WithLogger(&cfg.Logger)),
		keyPath:     cfg.KeyPath,
		cliVersion:  cfg.CLIversion,
		cliDigest:   cfg.CLIDigest,
	}
}

// TODO: Return defined type
func (action *AttestationPush) Run() (interface{}, error) {
	if initialized := action.c.AlreadyInitialized(); !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := action.c.LoadCraftingState(); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	if err := action.c.ValidateAttestation(); err != nil {
		return nil, err
	}

	action.Logger.Debug().Msg("validation completed")

	renderer, err := renderer.NewAttestationRenderer(action.c.CraftingState, action.keyPath, action.cliVersion, action.cliDigest, &action.Logger)
	if err != nil {
		return nil, err
	}

	res, err := renderer.Render()
	if err != nil {
		return nil, err
	}

	action.Logger.Debug().Msg("render completed")
	if action.c.CraftingState.DryRun {
		action.Logger.Info().Msg("dry-run completed, push skipped")
		// We are done, remove the existing att state
		if err := action.c.Reset(); err != nil {
			return nil, err
		}
		return res, nil
	}

	if err := pushToControlPlane(action.ActionsOpts.CPConnection, res, action.c.CraftingState.Attestation.GetWorkflow().GetWorkflowRunId()); err != nil {
		return nil, err
	}

	action.Logger.Info().Msg("push completed of the following payload")

	// We are done, remove the existing att state
	if err := action.c.Reset(); err != nil {
		return nil, err
	}

	return res, nil
}

func pushToControlPlane(conn *grpc.ClientConn, envelope *dsse.Envelope, workflowRunID string) error {
	encodedAttestation, err := encodeEnvelope(envelope)
	if err != nil {
		return err
	}

	client := pb.NewAttestationServiceClient(conn)
	if _, err := client.Store(context.Background(), &pb.AttestationServiceStoreRequest{
		Attestation:   encodedAttestation,
		WorkflowRunId: workflowRunID,
	}); err != nil {
		return err
	}

	return nil
}

func encodeEnvelope(e *dsse.Envelope) ([]byte, error) {
	return json.Marshal(e)
}

func decodeEnvelope(rawEnvelope []byte) (*dsse.Envelope, error) {
	envelope := &dsse.Envelope{}
	if err := json.Unmarshal(rawEnvelope, envelope); err != nil {
		return nil, err
	}

	return envelope, nil
}
