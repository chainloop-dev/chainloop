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

package bearertoken

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireTransportSecurity(t *testing.T) {
	testCases := []struct {
		name     string
		insecure bool
		expected bool
	}{
		{
			name:     "secure",
			insecure: false,
			expected: true,
		}, {
			name:     "insecure",
			insecure: true,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ta := NewTokenAuth("token", tc.insecure)
			assert.Equal(t, tc.expected, ta.RequireTransportSecurity())
		})
	}
}

func TestGetRequestMetadata(t *testing.T) {
	ta := NewTokenAuth("token", false)
	metadata, err := ta.GetRequestMetadata(context.TODO())
	assert.NoError(t, err)
	assert.Equal(t, metadata, map[string]string{
		"authorization": "Bearer token",
	})
}
