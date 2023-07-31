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

package dependencytrack

import (
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
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
			name:   "missing instance URL",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'instanceURI'",
		},
		{
			name:   "invalid instance URL",
			input:  map[string]interface{}{"instanceURI": "localhost"},
			errMsg: "is not valid 'uri'",
		},
		{
			name:   "missing API key",
			input:  map[string]interface{}{"instanceURI": "https://foo.com"},
			errMsg: "missing properties: 'apiKey'",
		},
		{
			name:  "valid request",
			input: map[string]interface{}{"instanceURI": "https://foo.com", "apiKey": "api-key"},
		},
		{
			name:  "valid request with path",
			input: map[string]interface{}{"instanceURI": "https://foo.com:3000/path", "apiKey": "api-key"},
		},
		{
			name:  "valid request with allowAutoCreate",
			input: map[string]interface{}{"instanceURI": "https://foo.com:3000/path", "apiKey": "api-key", "allowAutoCreate": true},
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

func TestResolveProjectName(t *testing.T) {
	testCases := []struct {
		name        string
		projectName string
		wantErr     bool
		want        string
	}{
		{
			name:        "no interpolation",
			projectName: "hi",
			want:        "hi",
			wantErr:     false,
		},
		{
			name:        "no interpolation",
			projectName: "{.Hello}",
			want:        "{.Hello}",
			wantErr:     false,
		},
		{
			name:        "nope",
			projectName: "{.Hello",
			want:        "{.Hello",
			wantErr:     false,
		},
		{
			name:        "invalid template",
			projectName: "{{.Hello",
			wantErr:     true,
		},
		{
			name:        "interpolated key",
			projectName: "{{.Material.Annotations.Hello}}",
			want:        "hola",
		},
		{
			name:        "interpolated string",
			projectName: "{{.Material.Annotations.Hello}}-project",
			want:        "hola-project",
		},
		{
			name:        "non-existing",
			projectName: "{{.Material.Annotations.noVal}}",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "lower case",
			projectName: "{{.Material.Annotations.hello}}",
			want:        "hola",
		},
		{
			name:        "interpolated string global",
			projectName: "project-{{.Attestation.Annotations.version}}",
			want:        "project-1.2.3",
		},
		{
			name:        "interpolated combination global",
			projectName: "project-{{.Material.Annotations.Hello}}-{{.Attestation.Annotations.version}}",
			want:        "project-hola-1.2.3",
		},
		{
			name:        "uppercase",
			projectName: "{{.Attestation.Annotations.Version}}",
			want:        "1.2.3",
		},
	}

	sbomAnnotation := map[string]string{
		"Hello": "hola",
		"World": "mundo",
	}

	attAnnotation := map[string]string{
		"version": "1.2.3",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			got, err := resolveProjectName(tc.projectName, attAnnotation, sbomAnnotation)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
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
			name:   "missing project info",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'projectName'",
		},
		{
			name:  "valid request, project ID",
			input: map[string]interface{}{"projectID": "project-id"},
		},
		{
			name:  "valid request with name",
			input: map[string]interface{}{"projectName": "project-name"},
		},
		{
			name:   "invalid with both",
			input:  map[string]interface{}{"projectID": "project-id", "projectName": "project-name"},
			errMsg: "valid against schemas at indexes 0 and 1",
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

func TestValidateExecuteOpts(t *testing.T) {
	validMaterial := &sdk.ExecuteMaterial{NormalizedMaterial: &chainloop.NormalizedMaterial{Type: "SBOM_CYCLONEDX_JSON"}, Content: []byte("content")}

	testCases := []struct {
		name   string
		opts   *sdk.ExecutionRequest
		errMsg string
	}{
		{
			name: "invalid - missing material",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{{}}},
			},
			errMsg: "invalid input",
		},
		{
			name: "invalid - invalid material",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{{NormalizedMaterial: &chainloop.NormalizedMaterial{Type: "invalid"}, Content: []byte("content")}}},
			},
			errMsg: "invalid input type",
		},
		{
			name: "invalid - missing configuration",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{validMaterial}},
			},
			errMsg: "missing registration configuration",
		},
		{
			name: "invalid - missing attachment configuration",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{validMaterial}},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config"), Credentials: &sdk.Credentials{Password: "password"}},
			},
			errMsg: "missing attachment configuration",
		},
		{
			name: "invalid - missing registration configuration",
			opts: &sdk.ExecutionRequest{
				Input:          &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{validMaterial}},
				AttachmentInfo: &sdk.AttachmentResponse{},
			},
			errMsg: "missing registration configuration",
		},
		{
			name: "invalid - missing credentials",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{validMaterial}},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config")},
				AttachmentInfo:   &sdk.AttachmentResponse{Configuration: []byte("config")},
			},
			errMsg: "missing credentials",
		},
		{
			name: "ok - all good",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Materials: []*sdk.ExecuteMaterial{validMaterial}},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config"), Credentials: &sdk.Credentials{Password: "password"}},
				AttachmentInfo:   &sdk.AttachmentResponse{Configuration: []byte("config")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.opts.Input.Materials[0]
			err := validateExecuteOpts(m, tc.opts.RegistrationInfo, tc.opts.AttachmentInfo)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
