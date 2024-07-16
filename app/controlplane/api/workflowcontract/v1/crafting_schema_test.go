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
	"errors"
	"testing"

	"github.com/bufbuild/protovalidate-go"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	validator, err := protovalidate.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			annotation := &v1.Annotation{
				Name:  tc.name,
				Value: tc.value,
			}

			err := validator.Validate(annotation)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestPolicyAttachment(t *testing.T) {
	testCases := []struct {
		desc      string
		policy    *v1.PolicyAttachment
		wantErr   bool
		violation string
	}{
		{
			desc:      "empty policy",
			policy:    &v1.PolicyAttachment{},
			wantErr:   true,
			violation: "policy",
		},
		{
			desc:    "policy ref",
			policy:  &v1.PolicyAttachment{Policy: &v1.PolicyAttachment_Ref{Ref: "reference"}},
			wantErr: false,
		},
		{
			desc: "incomplete arguments",
			policy: &v1.PolicyAttachment{
				Policy: &v1.PolicyAttachment_Ref{Ref: "reference"},
				With: []*v1.PolicyAttachment_PolicyArgument{
					{
						Name: "name",
					},
				},
			},
			wantErr:   true,
			violation: "with[0].value",
		},
		{
			desc: "complete arguments",
			policy: &v1.PolicyAttachment{
				Policy: &v1.PolicyAttachment_Ref{Ref: "reference"},
				With: []*v1.PolicyAttachment_PolicyArgument{
					{
						Name:  "name",
						Value: "value",
					},
				},
			},
			wantErr: false,
		},
	}

	validator, err := protovalidate.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validator.Validate(tc.policy)
			if tc.wantErr {
				assert.Error(t, err)

				valErr := &protovalidate.ValidationError{}
				errors.As(err, &valErr)
				assert.Equal(t, tc.violation, valErr.Violations[0].FieldPath)
				assert.Contains(t, err.Error(), tc.violation)

				return
			}
			assert.NoError(t, err)
		})
	}
}
