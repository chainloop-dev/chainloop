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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
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

type AttestationAdd struct {
	*ActionsOpts
	c      *crafter.Crafter
	casURI string
	// optional CA certificate for the CAS connection
	casCAPath          string
	connectionInsecure bool
}

func NewAttestationAdd(cfg *AttestationAddOpts) (*AttestationAdd, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	if cfg.RegistryServer != "" && cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		cfg.Logger.Debug().Str("server", cfg.RegistryServer).Str("username", cfg.RegistryUsername).Msg("using OCI registry credentials")
		opts = append(opts, crafter.WithOCIAuth(cfg.RegistryServer, cfg.RegistryUsername, cfg.RegistryPassword))
	}

	c, err := newCrafter(cfg.UseAttestationRemoteState, cfg.CPConnection, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	return &AttestationAdd{
		ActionsOpts:        cfg.ActionsOpts,
		c:                  c,
		casURI:             cfg.CASURI,
		casCAPath:          cfg.CASCAPath,
		connectionInsecure: cfg.ConnectionInsecure,
	}, nil
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(ctx context.Context, attestationID, materialName, materialValue, materialType string, annotations map[string]string) error {
	if initialized, err := action.c.AlreadyInitialized(ctx, attestationID); err != nil {
		return fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return ErrAttestationNotInitialized
	}

	if err := action.c.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return err
	}

	// Default to inline CASBackend and override if we are not in dry-run mode
	casBackend, closefunc, err := getCasBackend(ctx, action.c.CraftingState, action.ActionsOpts, action.casCAPath, action.casURI, action.connectionInsecure)
	if err != nil {
		return fmt.Errorf("getting cas backend: %w", err)
	}
	defer closefunc()

	// Add material to the attestation crafting state based on if the material is contract free or not.
	// By default, try to detect the material kind automatically
	switch {
	case materialName == "" && materialType == "":
		var kind schemaapi.CraftingSchema_Material_MaterialType
		if kind, err = action.c.AddMaterialContactFreeAutomatic(ctx, attestationID, materialValue, casBackend, annotations); err != nil {
			return fmt.Errorf("adding material: %w", err)
		}
		action.Logger.Info().Str("kind", kind.String()).Msg("material kind detected")
	case materialName != "":
		err = action.c.AddMaterialFromContract(ctx, attestationID, materialName, materialValue, casBackend, annotations)
	default:
		err = action.c.AddMaterialContractFree(ctx, attestationID, materialType, materialValue, casBackend, annotations)
	}

	if err != nil {
		return fmt.Errorf("adding material: %w", err)
	}

	return nil
}
