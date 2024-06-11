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

package data

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEntCASBackendTo(t *testing.T) {
	testBackend := ent.CASBackend{
		ID:          uuid.New(),
		Location:    "test-repo",
		Provider:    "test-provider",
		SecretName:  "test-secret",
		Description: "test-description",
		CreatedAt:   time.Now(),
		Default:     true,
	}

	testBackendInline := testBackend
	testBackendInline.Provider = "INLINE"

	testBackendFallback := testBackend
	testBackendFallback.Fallback = true

	tests := []struct {
		input  *ent.CASBackend
		output *biz.CASBackend
	}{
		{nil, nil},
		{&testBackend, &biz.CASBackend{
			ID:          testBackend.ID,
			Location:    testBackend.Location,
			SecretName:  testBackend.SecretName,
			Description: testBackend.Description,
			CreatedAt:   toTimePtr(testBackend.CreatedAt),
			Provider:    testBackend.Provider,
			Default:     true,
			Limits: &biz.CASBackendLimits{
				MaxBytes: biz.CASBackendDefaultMaxBytes,
			},
		}},
		{&testBackendInline, &biz.CASBackend{
			ID:          testBackend.ID,
			Location:    testBackend.Location,
			SecretName:  testBackend.SecretName,
			Description: testBackend.Description,
			CreatedAt:   toTimePtr(testBackend.CreatedAt),
			Provider:    "INLINE",
			Default:     true,
			Inline:      true,
			Limits: &biz.CASBackendLimits{
				MaxBytes: biz.CASBackendInlineDefaultMaxBytes,
			},
		}},
		{&testBackendFallback, &biz.CASBackend{
			ID:          testBackend.ID,
			Location:    testBackend.Location,
			SecretName:  testBackend.SecretName,
			Description: testBackend.Description,
			CreatedAt:   toTimePtr(testBackend.CreatedAt),
			Provider:    testBackend.Provider,
			Default:     true,
			Fallback:    true,
			Limits: &biz.CASBackendLimits{
				MaxBytes: biz.CASBackendDefaultMaxBytes,
			},
		}},
	}

	for _, tc := range tests {
		got := entCASBackendToBiz(tc.input)
		assert.Equal(t, tc.output, got)
	}
}
