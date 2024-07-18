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

package action

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer"
	"github.com/chainloop-dev/chainloop/internal/attestation/signer"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AttestationPushOpts struct {
	*ActionsOpts
	KeyPath, CLIVersion, CLIDigest, BundlePath string

	SignServerCAPath string
}

type AttestationResult struct {
	Digest   string                   `json:"digest"`
	Envelope *dsse.Envelope           `json:"envelope"`
	Status   *AttestationStatusResult `json:"status"`
}

type AttestationPush struct {
	*ActionsOpts
	keyPath, cliVersion, cliDigest, bundlePath string
	signServerCAPath                           string
	*newCrafterOpts
}

func NewAttestationPush(cfg *AttestationPushOpts) (*AttestationPush, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	return &AttestationPush{
		ActionsOpts:      cfg.ActionsOpts,
		keyPath:          cfg.KeyPath,
		cliVersion:       cfg.CLIVersion,
		cliDigest:        cfg.CLIDigest,
		bundlePath:       cfg.BundlePath,
		signServerCAPath: cfg.SignServerCAPath,
		newCrafterOpts:   &newCrafterOpts{cpConnection: cfg.CPConnection, opts: opts},
	}, nil
}

func (action *AttestationPush) Run(ctx context.Context, attestationID string, runtimeAnnotations map[string]string) (*AttestationResult, error) {
	useRemoteState := attestationID != ""
	// initialize the crafter. If attestation-id is provided we assume the attestation is performed using remote state
	crafter, err := newCrafter(useRemoteState, action.CPConnection, action.newCrafterOpts.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	// Retrieve attestation status
	statusAction, err := NewAttestationStatus(&AttestationStatusOpts{ActionsOpts: action.ActionsOpts, UseAttestationRemoteState: useRemoteState})
	if err != nil {
		return nil, fmt.Errorf("creating status action: %w", err)
	}
	attestationStatus, err := statusAction.Run(ctx, attestationID)
	if err != nil {
		return nil, fmt.Errorf("creating running status action: %w", err)
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	// Annotations
	craftedAnnotations := make(map[string]string, 0)
	// 1 - Set annotations that come from the contract
	for _, v := range crafter.CraftingState.InputSchema.GetAnnotations() {
		craftedAnnotations[v.Name] = v.Value
	}

	// 2 - Populate annotation values from the ones provided at runtime
	// a) we do not allow overriding values that come from the contract
	// b) we do not allow adding annotations that are not defined in the contract
	for kr, vr := range runtimeAnnotations {
		// If the annotation is not defined in the material we fail
		if v, found := craftedAnnotations[kr]; !found {
			return nil, fmt.Errorf("annotation %q not found", kr)
		} else if v == "" {
			// Set it only if it's not set
			craftedAnnotations[kr] = vr
		} else {
			// NOTE: we do not allow overriding values that come from the contract
			action.Logger.Info().Str("annotation", kr).Msg("annotation can't be changed, skipping")
		}
	}

	// Make sure all the annotation values are now set
	// This is in fact validated below but by manually checking we can provide a better error message
	for k, v := range craftedAnnotations {
		var missingAnnotations []string
		if v == "" {
			missingAnnotations = append(missingAnnotations, k)
		}

		if len(missingAnnotations) > 0 {
			return nil, fmt.Errorf("annotations %q required", missingAnnotations)
		}
	}
	// Set the annotations
	crafter.CraftingState.Attestation.Annotations = craftedAnnotations

	if err := crafter.ValidateAttestation(); err != nil {
		return nil, err
	}

	action.Logger.Debug().Msg("validation completed")

	// Indicate that we are done with the attestation
	crafter.CraftingState.Attestation.FinishedAt = timestamppb.New(time.Now())

	sig, err := signer.GetSigner(action.keyPath, action.Logger, &signer.Opts{
		SignServerCAPath: action.signServerCAPath,
		Vaultclient:      pb.NewSigningServiceClient(action.CPConnection),
	})
	if err != nil {
		return nil, fmt.Errorf("creating signer: %w", err)
	}

	renderer, err := renderer.NewAttestationRenderer(crafter.CraftingState, action.cliVersion, action.cliDigest, sig,
		renderer.WithLogger(action.Logger), renderer.WithBundleOutputPath(action.bundlePath))
	if err != nil {
		return nil, err
	}

	envelope, err := renderer.Render(ctx)
	if err != nil {
		return nil, err
	}

	attestationResult := &AttestationResult{Envelope: envelope, Status: attestationStatus}

	action.Logger.Debug().Msg("render completed")
	if crafter.CraftingState.DryRun {
		action.Logger.Info().Msg("dry-run completed, push skipped")
		// We are done, remove the existing att state
		if err := crafter.Reset(ctx, attestationID); err != nil {
			return nil, err
		}

		return attestationResult, nil
	}

	attestationResult.Digest, err = pushToControlPlane(ctx, action.ActionsOpts.CPConnection, envelope, crafter.CraftingState.Attestation.GetWorkflow().GetWorkflowRunId())
	if err != nil {
		return nil, fmt.Errorf("pushing to control plane: %w", err)
	}

	action.Logger.Info().Msg("push completed")

	// We are done, remove the existing att state
	if err := crafter.Reset(ctx, attestationID); err != nil {
		return nil, err
	}

	return attestationResult, nil
}

func pushToControlPlane(ctx context.Context, conn *grpc.ClientConn, envelope *dsse.Envelope, workflowRunID string) (string, error) {
	encodedAttestation, err := encodeEnvelope(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding attestation: %w", err)
	}

	client := pb.NewAttestationServiceClient(conn)
	resp, err := client.Store(ctx, &pb.AttestationServiceStoreRequest{
		Attestation:   encodedAttestation,
		WorkflowRunId: workflowRunID,
	})

	if err != nil {
		return "", fmt.Errorf("contacting the control plane: %w", err)
	}

	return resp.Result.Digest, nil
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
