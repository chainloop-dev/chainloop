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
			desc: "complete arguments",
			policy: &v1.PolicyAttachment{
				Policy: &v1.PolicyAttachment_Ref{Ref: "reference"},
				With:   map[string]string{"foo": "bar"},
			},
			wantErr: false,
		},
		{
			desc: "valid requirements",
			policy: &v1.PolicyAttachment{
				Policy:       &v1.PolicyAttachment_Ref{Ref: "reference"},
				Requirements: []string{"foo", "foo@1.2.3", "foo_bar@PRODUCTION", "foo123@a.b", "1A-F4@__foo"},
			},
		},
		{
			desc: "invalid requirements",
			policy: &v1.PolicyAttachment{
				Policy:       &v1.PolicyAttachment_Ref{Ref: "reference"},
				Requirements: []string{"foo bar"},
			},
			violation: "requirements[0]",
			wantErr:   true,
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

func TestValidateRefs(t *testing.T) {
	testCases := []struct {
		name          string
		ref           string
		wantErrString string
	}{
		{
			name:          "empty",
			ref:           "",
			wantErrString: "empty",
		},
		{
			name: "valid absolute file path",
			ref:  "file://path/to/file.yaml",
		},
		{
			name: "valid relative file path",
			ref:  "file://../path/to/file.yaml",
		},
		{
			name: "valid file path with digest",
			ref:  "file://path/to/file.yaml@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
		},
		{
			name:          "invalid sha256",
			ref:           "file://path/to/file.yaml@sha256:deadbeef",
			wantErrString: "invalid digest",
		},
		{
			name:          "invalid sha256 prefix missing",
			ref:           "file://path/to/file.yaml@b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
			wantErrString: "invalid digest",
		},
		{
			name:          "file with no extension",
			ref:           "file://path/to/file",
			wantErrString: "missing extension",
		},
		{
			name: "file with no path",
			ref:  "file://file.yaml",
		},
		{
			name:          "file with no path and no extension",
			ref:           "file://file",
			wantErrString: "missing extension",
		},
		{
			name:          "file with no path nor host",
			ref:           "file://",
			wantErrString: "invalid file reference",
		},
		{
			name: "valid http URL",
			ref:  "http://example.com/path/to/file.yaml",
		},
		{
			name: "valid https URL",
			ref:  "https://example.com/path/to/file.yaml",
		},
		{
			name:          "invalid http with sha256",
			ref:           "https://example.com/path/to/file.yaml@sha256:deafbeef",
			wantErrString: "invalid digest",
		},
		{
			name:          "http URL with no extension",
			ref:           "https://example.com/path/to/file",
			wantErrString: "missing extension",
		},
		{
			name: "valid chainloop protocol",
			ref:  "chainloop://policy-name",
		},
		{
			name: "valid chainloop protocol with provider and policy name",
			ref:  "chainloop://foo:policy-name",
		},
		{
			name: "valid implicit protocol with just policy name",
			ref:  "policy-name",
		},
		{
			name: "valid implicit protocol with both provider and policy name",
			ref:  "foo:policy-name",
		},
		{
			name:          "invalid provider name",
			ref:           "fooBar:policy-name",
			wantErrString: "invalid provider name",
		},
		{
			name:          "invalid policy name",
			ref:           "foobar:policy_name",
			wantErrString: "invalid policy name",
		},
		{
			name:          "invalid digest",
			ref:           "foobar:policy_name@foobar",
			wantErrString: "invalid digest",
		},
		{
			name: "valid digest",
			ref:  "foobar:policy-name@sha256:133d39edc0f0d32780dd9c940951df0910ef53e6fd64942801ba6fb76494bbf9",
		},
		{
			name: "chainloop provider with valid digest",
			ref:  "foobar:policy-name@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
		},
		{
			name: "custom policy with valid digest",
			ref:  "readonly-demo/policy-name@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
		},
		{
			name: "builtin policy with valid digest",
			ref:  "policy-name@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
		},
		{
			name:          "unsupported protocol",
			ref:           "unsupported://foobar/policy_name",
			wantErrString: "unsupported protocol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := v1.ValidatePolicyAttachmentRef(tc.ref)
			if tc.wantErrString != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrString)
				return
			}

			require.NoError(t, err)
		})
	}
}
