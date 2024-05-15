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

package posthog_test

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry/posthog"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	testCases := []struct {
		name     string
		apiKey   string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "empty api key",
			endpoint: "random-endpoint",
			wantErr:  true,
		},
		{
			name:    "empty endpoint",
			apiKey:  "random-api-key",
			wantErr: true,
		},
		{
			name:     "valid api key and endpoint",
			apiKey:   "random-api-key",
			endpoint: "random-endpoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl, err := posthog.NewClient(tc.apiKey, tc.endpoint)
			if tc.wantErr {
				assert.Nil(t, cl)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, cl)
				assert.NoError(t, err)
			}
		})
	}
}
