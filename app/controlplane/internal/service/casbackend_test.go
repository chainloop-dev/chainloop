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

package service

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestBizCASBackendToPb_HidesManagedDetails guards the rule that
// managed-backend implementation details (AP ARN, provider ID) never
// leak to API clients. Non-managed rows keep their original Location
// and Provider; managed rows are rewritten to stable placeholders.
// Regression-prevention only — both fields are otherwise straightforward
// to map.
func TestBizCASBackendToPb_HidesManagedDetails(t *testing.T) {
	now := time.Now()
	realLocation := "arn:aws:s3:us-east-1:471112941097:accesspoint/chainloop-org-dev"
	realProvider := biz.CASBackendProvider("AWS-S3-ACCESS-POINT")

	base := biz.CASBackend{
		ID:          uuid.New(),
		Name:        "backend",
		Location:    realLocation,
		CreatedAt:   &now,
		UpdatedAt:   &now,
		ValidatedAt: &now,
		Provider:    realProvider,
	}

	t.Run("non-managed exposes location and provider verbatim", func(t *testing.T) {
		in := base
		in.Managed = false
		got := bizCASBackendToPb(&in)
		assert.Equal(t, realLocation, got.Location,
			"non-managed rows must surface their real location")
		assert.Equal(t, string(realProvider), got.Provider,
			"non-managed rows must surface their real provider")
		assert.False(t, got.IsManaged)
	})

	t.Run("managed replaces location and provider with placeholders", func(t *testing.T) {
		in := base
		in.Managed = true
		got := bizCASBackendToPb(&in)
		assert.Equal(t, biz.CASBackendManagedLocationDisplay, got.Location,
			"managed rows must never leak the underlying AP ARN")
		assert.Equal(t, biz.CASBackendManagedProviderDisplay, got.Provider,
			"managed rows must never leak the backing provider ID")
		assert.True(t, got.IsManaged)
	})
}
