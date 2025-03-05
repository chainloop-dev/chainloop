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
	"os"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/signer"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AttestationPushOpts struct {
	*ActionsOpts
	KeyPath, CLIVersion, CLIDigest, BundlePath string

	LocalStatePath string
	SignServerOpts *SignServerOpts
}

// SignServerOpts holds SignServer integration options
type SignServerOpts struct {
	// CA certificate for TLS connection
	CAPath string
	// (optional) Client cert and passphrase for mutual TLS authentication
	AuthClientCertPath, AuthClientCertPass string
}

type AttestationResult struct {
	Digest   string                   `json:"digest"`
	Envelope *dsse.Envelope           `json:"envelope"`
	Status   *AttestationStatusResult `json:"status"`
}

type AttestationPush struct {
	*ActionsOpts
	keyPath, cliVersion, cliDigest, bundlePath string
	localStatePath                             string
	signServerOpts                             *SignServerOpts
	*newCrafterOpts
}

func NewAttestationPush(cfg *AttestationPushOpts) (*AttestationPush, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	return &AttestationPush{
		ActionsOpts:    cfg.ActionsOpts,
		keyPath:        cfg.KeyPath,
		cliVersion:     cfg.CLIVersion,
		cliDigest:      cfg.CLIDigest,
		bundlePath:     cfg.BundlePath,
		signServerOpts: cfg.SignServerOpts,
		localStatePath: cfg.LocalStatePath,
		newCrafterOpts: &newCrafterOpts{cpConnection: cfg.CPConnection, opts: opts},
	}, nil
}

func (action *AttestationPush) Run(ctx context.Context, attestationID string, runtimeAnnotations map[string]string, bypassPolicyCheck bool) (*AttestationResult, error) {
	useRemoteState := attestationID != ""
	// initialize the crafter. If attestation-id is provided we assume the attestation is performed using remote state
	crafter, err := newCrafter(&newCrafterStateOpts{enableRemoteState: useRemoteState, localStatePath: action.localStatePath}, action.CPConnection, action.newCrafterOpts.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	// Retrieve attestation status
	statusAction, err := NewAttestationStatus(&AttestationStatusOpts{
		ActionsOpts: action.ActionsOpts, UseAttestationRemoteState: useRemoteState, isPushed: true, LocalStatePath: action.localStatePath,
	})

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
	crafter.CraftingState.Attestation.BypassPolicyCheck = bypassPolicyCheck

	signerOpts := &signer.Opts{Vaultclient: pb.NewSigningServiceClient(action.CPConnection)}
	if action.signServerOpts != nil {
		signerOpts.SignServerOpts = &signer.SignServerOpts{
			CAPath:             action.signServerOpts.CAPath,
			AuthClientCertPath: action.signServerOpts.AuthClientCertPath,
			AuthClientCertPass: action.signServerOpts.AuthClientCertPass,
		}
	}
	sig, err := signer.GetSigner(action.keyPath, action.Logger, signerOpts)
	if err != nil {
		return nil, fmt.Errorf("creating signer: %w", err)
	}

	attClient := pb.NewAttestationServiceClient(action.CPConnection)
	renderer, err := renderer.NewAttestationRenderer(crafter.CraftingState, attClient, action.cliVersion, action.cliDigest, sig,
		renderer.WithLogger(action.Logger), renderer.WithBundleOutputPath(action.bundlePath))
	if err != nil {
		return nil, err
	}

	// execute policy evaluations
	// We do not want to evaluate policies here during render since we want to do it in a separate step
	statement, err := renderer.RenderStatement(ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering statement: %w", err)
	}

	// Add attestation-level policy evaluations
	if err := crafter.EvaluateAttestationPolicies(ctx, attestationID, statement); err != nil {
		return nil, fmt.Errorf("evaluating attestation policies: %w", err)
	}

	// render final attestation with all the evaluated policies inside
	envelope, bundle, err := renderer.Render(ctx)
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

	workflow := crafter.CraftingState.Attestation.GetWorkflow()

	attestationResult.Digest, err = pushToControlPlane(ctx, action.ActionsOpts.CPConnection, envelope, bundle, workflow.GetWorkflowRunId(), workflow.GetVersion().GetMarkAsReleased())
	if err != nil {
		return nil, fmt.Errorf("pushing to control plane: %w", err)
	}

	action.Logger.Info().Msg("push completed")

	// Save bundle to disk
	if err = action.saveBundle(bundle); err != nil {
		return nil, fmt.Errorf("saving bundle: %w", err)
	}

	// We are done, remove the existing att state
	if err := crafter.Reset(ctx, attestationID); err != nil {
		return nil, err
	}

	return attestationResult, nil
}

func (action *AttestationPush) saveBundle(bundle *protobundle.Bundle) error {
	// Save bundle to disk if requested
	if action.bundlePath != "" {
		bundleJSON, err := encodeBundle(bundle)
		if err != nil {
			return fmt.Errorf("encoding bundle: %w", err)
		}
		action.Logger.Info().Msg(fmt.Sprintf("Storing Sigstore bundle %s", action.bundlePath))
		err = os.WriteFile(action.bundlePath, bundleJSON, 0600)
		if err != nil {
			return fmt.Errorf("writing bundle: %w", err)
		}
	}

	return nil
}

func pushToControlPlane(ctx context.Context, conn *grpc.ClientConn, envelope *dsse.Envelope, bundle *protobundle.Bundle, workflowRunID string, markVersionAsReleased bool) (string, error) {
	encodedBundle, err := encodeBundle(bundle)
	if err != nil {
		return "", fmt.Errorf("encoding attestation: %w", err)
	}

	client := pb.NewAttestationServiceClient(conn)

	// if endpoint doesn't accept the bundle, we still send the plain attestation for backwards compatibility
	encodedAttestation, err := encodeEnvelope(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding attestation: %w", err)
	}

	// remove additional base64 encoding in signature. See https://github.com/chainloop-dev/chainloop/issues/1832
	attestation.FixSignatureInBundle(bundle)
	encodedFixedBundle, err := encodeBundle(bundle)
	if err != nil {
		return "", fmt.Errorf("encoding attestation: %w", err)
	}

	// Store bundle next versions will perform this in a single call)
	resp, err := client.Store(ctx, &pb.AttestationServiceStoreRequest{
		Attestation:           encodedAttestation,
		Bundle:                encodedBundle,
		AttestationBundle:     encodedFixedBundle,
		WorkflowRunId:         workflowRunID,
		MarkVersionAsReleased: &markVersionAsReleased,
	})
	if err != nil {
		return "", fmt.Errorf("storing attestation: %w", err)
	}

	return resp.Result.Digest, nil
}

func encodeEnvelope(e *dsse.Envelope) ([]byte, error) {
	return json.Marshal(e)
}

func encodeBundle(b *protobundle.Bundle) ([]byte, error) {
	return protojson.Marshal(b)
}

func decodeEnvelope(rawEnvelope []byte) (*dsse.Envelope, error) {
	envelope := &dsse.Envelope{}
	if err := json.Unmarshal(rawEnvelope, envelope); err != nil {
		return nil, err
	}

	return envelope, nil
}
