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

package ociregistry

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
			name:   "not ok, missing properties",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'repository', 'username', 'password'",
		},
		{
			name:   "not ok, random properties",
			input:  map[string]interface{}{"foo": "bar"},
			errMsg: "additionalProperties 'foo' not allowed",
		},
		{
			name:  "ok, all properties",
			input: map[string]interface{}{"repository": "repo.io", "username": "u", "password": "p"},
		},
		{
			name:   "not ok, empty repository",
			input:  map[string]interface{}{"repository": "", "username": "u", "password": "p"},
			errMsg: "length must be >= 1, but got 0",
		},
		{
			name:   "not ok, empty username",
			input:  map[string]interface{}{"repository": "r", "username": "", "password": "p"},
			errMsg: "length must be >= 1, but got 0",
		},
		{
			name:   "not ok, empty password",
			input:  map[string]interface{}{"repository": "r", "username": "u", "password": ""},
			errMsg: "length must be >= 1, but got 0",
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
			name:  "ok, no properties",
			input: map[string]interface{}{},
		},
		{
			name:   "not ok, random properties",
			input:  map[string]interface{}{"foo": "bar"},
			errMsg: "additionalProperties 'foo' not allowed",
		},
		{
			name:   "not ok, set but empty prefix",
			input:  map[string]interface{}{"prefix": ""},
			errMsg: "length must be >= 1, but got 0",
		},
		{
			name:  "valid request",
			input: map[string]interface{}{"prefix": "p"},
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
