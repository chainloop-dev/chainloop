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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/statemanager/filesystem"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

type ActionsOpts struct {
	CPConnection *grpc.ClientConn
	Logger       zerolog.Logger
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}

// load a crafter with local state manager
// TODO: We'll enable the ability to load a crafter that relies on a remote state manager
func newCrafter(_ *grpc.ClientConn, logger *zerolog.Logger) (*crafter.Crafter, error) {
	statePath := filepath.Join(os.TempDir(), "chainloop-attestation.tmp.json")
	localStateManager, err := filesystem.New(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create local state manager: %w", err)
	}

	return crafter.NewCrafter(localStateManager, crafter.WithLogger(logger))
}
