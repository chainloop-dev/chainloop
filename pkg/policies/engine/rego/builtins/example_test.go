//
// Copyright 2025 The Chainloop Authors.
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

package builtins

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelloBuiltin(t *testing.T) {
	tests := []struct {
		name            string
		policy          string
		mockErr         error
		expectedMessage string
		expectError     bool
	}{
		{
			name: "successful render",
			policy: `package test
import rego.v1

result := chainloop.hello("world")`,
			expectedMessage: "Hello, world!",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, RegisterHelloBuiltin())
			// Prepare rego evaluation
			ctx := context.Background()
			r := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", tt.policy),
			)
			rs, err := r.Eval(ctx)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, rs, 1)
			require.Len(t, rs[0].Expressions, 1)

			result, ok := rs[0].Expressions[0].Value.(map[string]interface{})
			require.True(t, ok)

			// The status is returned as a number, convert it appropriately
			msgVal := result["message"]
			assert.Equal(t, tt.expectedMessage, msgVal)
		})
	}
}
