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

package smtp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRegistrationInput(t *testing.T) {
	testCases := []struct {
		name   string
		input  map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing properties",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'to', 'from', 'user'",
		},
		{
			name:  "valid request",
			input: map[string]interface{}{"from": "test@example.com", "to": "test@example.com", "host": "smtp.service.example.com", "port": "25", "user": "test", "password": "test"},
		},
		{
			name:   "invalid email",
			input:  map[string]interface{}{"from": "testexample.com", "to": "test@example.com", "host": "smtp.service.example.com", "port": "25", "user": "test", "password": "test"},
			errMsg: "is not valid 'email'",
		},
	}

	integration, err := New(nil)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)
			err = integration.ValidateRegistrationRequest(payload)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAttachmentInput(t *testing.T) {
	testCases := []struct {
		name   string
		input  map[string]interface{}
		errMsg string
	}{
		{
			name:  "valid with no optional cc",
			input: map[string]interface{}{},
		},
		{
			name:  "valid cc format",
			input: map[string]interface{}{"cc": "test@example.com"},
		},
		{
			name:   "invalid cc format",
			input:  map[string]interface{}{"cc": "testexample.com"},
			errMsg: "is not valid 'email'",
		},
	}

	integration, err := New(nil)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)
			err = integration.ValidateAttachmentRequest(payload)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewIntegration(t *testing.T) {
	_, err := New(nil)
	assert.NoError(t, err)
}
