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

// limitations under the License.

package v1_test

import (
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
)

func TestValidateAnnotations(t *testing.T) {
	testCases := []struct {
		desc    string
		name    string
		value   string
		wantErr bool
	}{
		{
			desc:  "valid keys",
			name:  "hi",
			value: "hi",
		},
		{
			desc:    "missing key",
			value:   "hi",
			wantErr: true,
		},
		{
			desc:  "valid key underscore",
			name:  "hello_world",
			value: "hello_world",
		},
		{
			desc:    "invalid key hyphen",
			name:    "hello-world",
			value:   "hello-world",
			wantErr: true,
		},
		{
			desc:    "invalid key space",
			name:    " hello",
			value:   "hello-world",
			wantErr: true,
		},
		{
			desc:  "valid key camel case",
			name:  "helloWorld",
			value: "hello-world",
		},
		{
			desc:  "valid content",
			name:  "hello",
			value: "hello world this has spaces-and-hyphens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			annotation := &v1.Annotation{
				Name:  tc.name,
				Value: tc.value,
			}

			err := annotation.ValidateAll()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
