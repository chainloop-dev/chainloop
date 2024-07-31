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
	"errors"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"
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
	*newCrafterOpts
}

func NewAttestationAdd(cfg *AttestationAddOpts) (*AttestationAdd, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	if cfg.RegistryServer != "" && cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		cfg.Logger.Debug().Str("server", cfg.RegistryServer).Str("username", cfg.RegistryUsername).Msg("using OCI registry credentials")
		opts = append(opts, crafter.WithOCIAuth(cfg.RegistryServer, cfg.RegistryUsername, cfg.RegistryPassword))
	}

	return &AttestationAdd{
		ActionsOpts:        cfg.ActionsOpts,
		newCrafterOpts:     &newCrafterOpts{cpConnection: cfg.CPConnection, opts: opts},
		casURI:             cfg.CASURI,
		casCAPath:          cfg.CASCAPath,
		connectionInsecure: cfg.ConnectionInsecure,
	}, nil
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(ctx context.Context, attestationID, materialName, materialValue, materialType string, annotations map[string]string) error {
	// initialize the crafter. If attestation-id is provided we assume the attestation is performed using remote state
	crafter, err := newCrafter(attestationID != "", action.CPConnection, action.newCrafterOpts.opts...)
	if err != nil {
		return fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return ErrAttestationNotInitialized
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return err
	}

	// Default to inline CASBackend and override if we are not in dry-run mode
	casBackend := &casclient.CASBackend{
		Name: "not-set",
	}

	// Define CASbackend information based on the API response
	if !crafter.CraftingState.GetDryRun() {
		// Get upload creds and CASbackend for the current attestation and set up CAS client
		client := pb.NewAttestationServiceClient(action.CPConnection)
		creds, err := client.GetUploadCreds(ctx,
			&pb.AttestationServiceGetUploadCredsRequest{
				WorkflowRunId: crafter.CraftingState.GetAttestation().GetWorkflow().GetWorkflowRunId(),
			},
		)
		if err != nil {
			return err
		}
		b := creds.GetResult().GetBackend()
		if b == nil {
			return fmt.Errorf("no backend found in upload creds")
		}
		casBackend.Name = b.Provider
		casBackend.MaxSize = b.GetLimits().MaxBytes
		// Some CASBackends will actually upload information to the CAS server
		// in such case we need to set up a connection
		if !b.IsInline && creds.Result.Token != "" {
			var opts = []grpcconn.Option{
				grpcconn.WithInsecure(action.connectionInsecure),
			}

			if action.casCAPath != "" {
				opts = append(opts, grpcconn.WithCAFile(action.casCAPath))
			}

			artifactCASConn, err := grpcconn.New(action.casURI, creds.Result.Token, opts...)
			if err != nil {
				return err
			}
			defer artifactCASConn.Close()

			casBackend.Uploader = casclient.New(artifactCASConn, casclient.WithLogger(action.Logger))
		}
	}

	// Add material to the attestation crafting state based on if the material is contract free or not.
	// The checks are performed in the following order:
	// 1. If materialName is empty and materialType is empty, we don't know anything about the material so, we add it with auto-detected kind and random name
	// 2. If materialName is not empty, check if the material is in the contract. If it is, add material from contract
	// 2.1. If materialType is empty, try to guess the material kind with auto-detected kind and materialName
	// 3. If materialType is not empty, add material contract free with materialType and materialName
	var kind schemaapi.CraftingSchema_Material_MaterialType
	switch {
	case materialName == "" && materialType == "":
		kind, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, "", materialValue, casBackend, annotations)
		if err != nil {
			return fmt.Errorf("adding material: %w", err)
		}
		action.Logger.Info().Str("kind", kind.String()).Msg("material kind detected")
	case materialName != "":
		// If the material is in the contract, add it from the contract
		if crafter.IsMaterialInContract(materialName) {
			err = crafter.AddMaterialFromContract(ctx, attestationID, materialName, materialValue, casBackend, annotations)
		} else if materialType == "" {
			// If the material is not in the contract and the materialType is not provided, add material contract free with auto-detected kind, guessing the kind
			kind, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, materialName, materialValue, casBackend, annotations)
			if err != nil {
				return fmt.Errorf("adding material: %w", err)
			}
			action.Logger.Info().Str("kind", kind.String()).Msg("material kind detected")
		} else {
			// If the material is not in the contract and has a materialType, add material contract free with the provided materialType
			err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations)
		}
	default:
		err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations)
	}

	if err != nil {
		return fmt.Errorf("adding material: %w", err)
	}

	return nil
}
