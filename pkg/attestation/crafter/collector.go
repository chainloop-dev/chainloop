//
// Copyright 2026 The Chainloop Authors.
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

package crafter

import (
	"context"

	"github.com/chainloop-dev/chainloop/pkg/casclient"
)

// Collector auto-discovers and attaches evidence during attestation init.
// Each collector runs best-effort: failures are logged but never fail the attestation.
type Collector interface {
	// ID returns a unique identifier for this collector (used in logs).
	ID() string
	// Collect discovers data and adds materials to the attestation.
	// Returning nil means nothing was collected (no-op is expected).
	Collect(ctx context.Context, crafter *Crafter, attestationID string, casBackend *casclient.CASBackend) error
}
