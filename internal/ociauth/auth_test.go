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

package ociauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCredentials(t *testing.T) {
	testCases := []struct {
		name       string
		repo       string
		username   string
		password   string
		wantErr    bool
		wantServer string
	}{
		// invalid
		{"empty repo", "", "username", "password", true, ""},
		{"empty username", "repo", "", "password", true, ""},
		{"empty password", "repo", "username", "", true, ""},
		{"explicit repo domain with schema", "https://chainloop.dev/oci-repo", "username", "password", true, ""},
		{"repo with port", "chainloop.dev:port", "username", "password", true, ""},

		// Valid
		{"explicit repo domain", "chainloop.dev/oci-repo", "username", "password", false, "chainloop.dev"},
		{"implicit repo domain", "repo", "username", "password", false, "index.docker.io"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewCredentials(tc.repo, tc.username, tc.password)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantServer, c.(*Credentials).server)
				assert.Equal(t, "username", c.(*Credentials).username)
				assert.Equal(t, "password", c.(*Credentials).password)
			}
		})
	}
}
