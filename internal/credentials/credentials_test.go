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

package credentials_test

import (
	"testing"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/stretchr/testify/assert"
)

func TestValidateAPICreds(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name      string
		input     *credentials.APICreds
		wantError bool
	}{
		{"empty secret", &credentials.APICreds{}, true},
		{"missing host", &credentials.APICreds{Host: "", Key: "p"}, true},
		{"missing key", &credentials.APICreds{Host: "host", Key: ""}, true},
		{"valid creds", &credentials.APICreds{Host: "h", Key: "p"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.wantError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestValidateOCIKeyPair(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name      string
		input     *credentials.OCIKeypair
		wantError bool
	}{
		{"empty secret", &credentials.OCIKeypair{}, true},
		{"missing repo", &credentials.OCIKeypair{Username: "un", Password: "p"}, true},
		{"missing username", &credentials.OCIKeypair{Username: "", Password: "p", Repo: "repo"}, true},
		{"missing password", &credentials.OCIKeypair{Username: "u", Password: "", Repo: "repo"}, true},
		{"valid creds", &credentials.OCIKeypair{Username: "u", Password: "p", Repo: "repo"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.wantError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
