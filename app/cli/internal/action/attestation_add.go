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
	"errors"

	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"google.golang.org/grpc"
)

type AttestationAddOpts struct {
	*ActionsOpts
	ArtifactsCASConn *grpc.ClientConn
}

type AttestationAdd struct {
	*ActionsOpts
	c                *crafter.Crafter
	artifactsCASConn *grpc.ClientConn
}

func NewAttestationAdd(cfg *AttestationAddOpts) *AttestationAdd {
	return &AttestationAdd{
		ActionsOpts: cfg.ActionsOpts,
		c: crafter.NewCrafter(
			crafter.WithLogger(&cfg.Logger),
			crafter.WithUploader(casclient.New(cfg.ArtifactsCASConn, casclient.WithLogger(cfg.Logger))),
		),
		artifactsCASConn: cfg.ArtifactsCASConn,
	}
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(k, v string) error {
	if initialized := action.c.AlreadyInitialized(); !initialized {
		return ErrAttestationNotInitialized
	}

	if err := action.c.LoadCraftingState(); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return err
	}

	if err := action.c.AddMaterial(k, v); err != nil {
		action.Logger.Err(err).Msg("adding material")
		return err
	}

	return nil
}
