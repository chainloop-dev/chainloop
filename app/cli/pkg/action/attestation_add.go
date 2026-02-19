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

package action

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"google.golang.org/grpc"
)

type AttestationAddOpts struct {
	*ActionsOpts
	ArtifactsCASConn   *grpc.ClientConn
	CASURI             string
	CASCAPath          string // optional CA certificate for the CAS connection
	ConnectionInsecure bool
	// OCI registry credentials used for CONTAINER_IMAGE material type
	RegistryServer, RegistryUsername, RegistryPassword string
	LocalStatePath                                     string
	// NoStrictValidation skips strict schema validation
	NoStrictValidation bool
}

type newCrafterOpts struct {
	cpConnection *grpc.ClientConn
	opts         []crafter.NewOpt
}

type AttestationAdd struct {
	*ActionsOpts
	casURI string
	// optional CA certificate for the CAS connection
	casCAPath          string
	connectionInsecure bool
	localStatePath     string
	*newCrafterOpts
}

func NewAttestationAdd(cfg *AttestationAddOpts) (*AttestationAdd, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	if cfg.RegistryServer != "" && cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		cfg.Logger.Debug().Str("server", cfg.RegistryServer).Str("username", cfg.RegistryUsername).Msg("using OCI registry credentials")
		opts = append(opts, crafter.WithOCIAuth(cfg.RegistryServer, cfg.RegistryUsername, cfg.RegistryPassword))
	}
	if cfg.NoStrictValidation {
		opts = append(opts, crafter.WithNoStrictValidation(cfg.NoStrictValidation))
	}

	return &AttestationAdd{
		ActionsOpts:        cfg.ActionsOpts,
		newCrafterOpts:     &newCrafterOpts{cpConnection: cfg.CPConnection, opts: opts},
		casURI:             cfg.CASURI,
		casCAPath:          cfg.CASCAPath,
		connectionInsecure: cfg.ConnectionInsecure,
		localStatePath:     cfg.LocalStatePath,
	}, nil
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(ctx context.Context, attestationID, materialName, materialValue, materialType string, annotations map[string]string) (*AttestationStatusMaterial, error) {
	// initialize the crafter. If attestation-id is provided we assume the attestation is performed using remote state
	crafter, err := newCrafter(&newCrafterStateOpts{enableRemoteState: (attestationID != ""), localStatePath: action.localStatePath}, action.CPConnection, action.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	// Default to inline CASBackend and override if we are not in dry-run mode
	casBackend := &casclient.CASBackend{
		Name: "not-set",
	}

	// Define CASbackend information based on the API response
	if !crafter.CraftingState.GetDryRun() {
		client := pb.NewAttestationServiceClient(action.CPConnection)
		workflowRunID := crafter.CraftingState.GetAttestation().GetWorkflow().GetWorkflowRunId()
		_, connectionCloserFn, getCASBackendErr := getCASBackend(ctx, client, workflowRunID, action.casCAPath, action.casURI, action.connectionInsecure, action.Logger, casBackend)
		if getCASBackendErr != nil {
			return nil, fmt.Errorf("failed to get CAS backend: %w", getCASBackendErr)
		}
		if connectionCloserFn != nil {
			// nolint: errcheck
			defer connectionCloserFn()
		}
	}

	// Add material to the attestation crafting state based on if the material is contract free or not.
	// The checks are performed in the following order:
	// 1. If materialName is empty and materialType is empty, we don't know anything about the material so, we add it with auto-detected kind and random name
	// 2. If materialName is not empty, check if the material is in the contract. If it is, add material from contract
	// 2.1. If materialType is empty, try to guess the material kind with auto-detected kind and materialName
	// 3. If materialType is not empty, add material contract free with materialType and materialName
	var mt *api.Attestation_Material
	switch {
	case materialName == "" && materialType == "":
		mt, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, "", materialValue, casBackend, annotations)
		if err != nil {
			return nil, fmt.Errorf("adding material: %w", err)
		}
		action.Logger.Info().Str("kind", mt.MaterialType.String()).Msg("material kind detected")
	case materialName != "":
		switch {
		// If the material is in the contract, add it from the contract
		case crafter.IsMaterialInContract(materialName):
			mt, err = crafter.AddMaterialFromContract(ctx, attestationID, materialName, materialValue, casBackend, annotations)
		// If the material is not in the contract and the materialType is not provided, add material contract free with auto-detected kind, guessing the kind
		case materialType == "":
			mt, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, materialName, materialValue, casBackend, annotations)
			if err != nil {
				return nil, fmt.Errorf("adding material: %w", err)
			}
			action.Logger.Info().Str("kind", mt.MaterialType.String()).Msg("material kind detected")
		// If the material is not in the contract and has a materialType, add material contract free with the provided materialType
		default:
			mt, err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations)
		}
	default:
		mt, err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations)
	}

	if err != nil {
		return nil, fmt.Errorf("adding material: %w", err)
	}

	materialResult, err := attMaterialToAction(mt)
	if err != nil {
		return nil, fmt.Errorf("converting material to action: %w", err)
	}

	return materialResult, nil
}

// GetPolicyEvaluations is a Wrapper around the getPolicyEvaluations
func (action *AttestationAdd) GetPolicyEvaluations(ctx context.Context, attestationID string) (map[string][]*PolicyEvaluation, error) {
	crafter, err := newCrafter(&newCrafterStateOpts{enableRemoteState: (attestationID != ""), localStatePath: action.localStatePath}, action.CPConnection, action.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	policyEvaluations, _ := GetPolicyEvaluations(crafter)

	return policyEvaluations, nil
}
