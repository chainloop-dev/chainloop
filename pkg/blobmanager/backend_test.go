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

package backend_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
)

// TestWithRequestingOrg_RoundTrip pins the ctx-key contract relied on
// by managed CAS providers (e.g. s3accesspoint), which read the org
// UUID via backend.RequestingOrgFromContext to mint per-tenant STS
// sessions. Changing the key type or accessor without updating those
// providers would silently break the fail-closed path.
func TestWithRequestingOrg_RoundTrip(t *testing.T) {
	// Empty by default.
	assert.Empty(t, backend.RequestingOrgFromContext(context.Background()))

	ctx := backend.WithRequestingOrg(context.Background(), "org-abc")
	assert.Equal(t, "org-abc", backend.RequestingOrgFromContext(ctx))

	// Overwrite uses the most recent value — important so a middleware
	// that sets the org isn't silently overridden by a stale value
	// further down the stack.
	ctx = backend.WithRequestingOrg(ctx, "org-xyz")
	assert.Equal(t, "org-xyz", backend.RequestingOrgFromContext(ctx))
}
