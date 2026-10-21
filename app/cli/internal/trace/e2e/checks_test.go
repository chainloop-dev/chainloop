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

package e2e

import (
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckModel(t *testing.T) {
	cases := []struct {
		name           string
		model          string
		primary        string
		wantErr        bool
		wantNotCovered bool
	}{
		{name: "short name", model: "haiku", primary: "claude-haiku-5-5"},
		{name: "provider prefix", model: "openrouter/anthropic/claude-haiku-5.5", primary: "claude-haiku-5.5"},
		{name: "other model", model: "haiku", primary: "gpt-5", wantErr: true},
		{name: "auto mode", model: modelAuto, primary: "default", wantErr: true, wantNotCovered: true},
		{name: "auto mode with no primary model", model: modelAuto, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Session{Model: tc.model, Stored: &StoredSession{}}
			if tc.primary != "" {
				s.Stored.Evidence.Data.Model = &aicodingsession.Model{Primary: tc.primary}
			}

			err := checkModel(s)
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tc.wantNotCovered, errors.Is(err, errNotCovered))
		})
	}
}
