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
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/statemanager/filesystem"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/statemanager/remote"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type ActionsOpts struct {
	CPConnection *grpc.ClientConn
	Logger       zerolog.Logger
}

type OffsetPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
	TotalCount int `json:"totalCount"`
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}

// load a crafter with either local or remote state
func newCrafter(enableRemoteState bool, conn *grpc.ClientConn, opts ...crafter.NewOpt) (*crafter.Crafter, error) {
	var stateManager crafter.StateManager
	var err error

	// run opts to extract logger
	c := &crafter.Crafter{}
	for _, opt := range opts {
		_ = opt(c)
	}

	switch enableRemoteState {
	case true:
		stateManager, err = remote.New(pb.NewAttestationStateServiceClient(conn), c.Logger)
	case false:
		attestationStatePath := filepath.Join(os.TempDir(), "chainloop-attestation.tmp.json")
		if path := os.Getenv("CHAINLOOP_ATTESTATION_STATE_PATH"); path != "" {
			attestationStatePath = path
		}

		stateManager, err = filesystem.New(attestationStatePath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	attClient := pb.NewAttestationServiceClient(conn)

	return crafter.NewCrafter(stateManager, attClient, opts...)
}
