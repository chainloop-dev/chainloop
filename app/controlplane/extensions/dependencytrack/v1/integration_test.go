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

package integration

import (
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/dependencytrack/v1/api"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/stretchr/testify/assert"
)

func TestValidateRegistrationInput(t *testing.T) {
	testCases := []struct {
		name   string
		input  *api.RegistrationRequest
		errMsg string
	}{
		{
			name:   "missing instance URL",
			input:  &api.RegistrationRequest{},
			errMsg: "invalid RegistrationRequest.InstanceUri",
		},
		{
			name:   "invalid instance URL",
			input:  &api.RegistrationRequest{InstanceUri: "localhost"},
			errMsg: "invalid RegistrationRequest.InstanceUri",
		},
		{
			name:   "missing API key",
			input:  &api.RegistrationRequest{InstanceUri: "https://foo.com"},
			errMsg: "invalid RegistrationRequest.ApiKey",
		},
		{
			name:  "valid request",
			input: &api.RegistrationRequest{InstanceUri: "http://localhost:8080", ApiKey: "api-key"},
		},
		{
			name:  "valid request with path",
			input: &api.RegistrationRequest{InstanceUri: "http://localhost:8080/path", ApiKey: "api-key"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
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
		input  *api.AttachmentRequest
		errMsg string
	}{
		{
			name:   "missing project info",
			input:  &api.AttachmentRequest{},
			errMsg: "invalid AttachmentRequest.Project",
		},
		{
			name:  "valid request, project ID",
			input: &api.AttachmentRequest{Project: &api.AttachmentRequest_ProjectId{ProjectId: "project-id"}},
		},
		{
			name:  "valid request with name",
			input: &api.AttachmentRequest{Project: &api.AttachmentRequest_ProjectName{ProjectName: "project-name"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfiguration(t *testing.T) {
	testIntegrationConfig := func(allowAutoCreate bool) *registrationConfig {
		return &registrationConfig{
			AllowAutoCreate: allowAutoCreate,
			Domain:          "domain",
		}
	}

	testAttachmentConfig := func(projectID, projectName string) *api.AttachmentRequest {
		req := &api.AttachmentRequest{}
		if projectID != "" {
			req.Project = &api.AttachmentRequest_ProjectId{ProjectId: projectID}
		} else if projectName != "" {
			req.Project = &api.AttachmentRequest_ProjectName{ProjectName: projectName}
		}

		return req
	}

	tests := []struct {
		integrationConfig *registrationConfig
		attachmentConfig  *api.AttachmentRequest
		errorMsg          string
	}{
		{nil, nil, "invalid configuration"},
		// autocreate required but not supported
		{testIntegrationConfig(false), testAttachmentConfig("", "new-project"), "auto creation of projects is not supported in this integration"},
		// autocreate required and supported
		{testIntegrationConfig(true), testAttachmentConfig("", "new-project"), ""},
		// Neither projectID nor autocreate provided
		{testIntegrationConfig(false), testAttachmentConfig("", ""), "project id or name must be provided"},
		// project ID provided
		{testIntegrationConfig(false), testAttachmentConfig("pid", ""), ""},
	}

	for _, tc := range tests {
		t.Run(tc.errorMsg, func(t *testing.T) {
			err := validateAttachmentConfiguration(tc.integrationConfig, tc.attachmentConfig)
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestNewIntegration(t *testing.T) {
	i, err := NewIntegration(nil)
	assert.NoError(t, err)

	assert.Equal(t, &sdk.IntegrationInfo{
		ID:      "dependencytrack",
		Version: "1.0",
		SubscribedInputs: &sdk.Inputs{
			Materials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				},
			},
		},
	}, i.Describe())
}

func TestValidateExecuteOpts(t *testing.T) {
	validMaterial := &sdk.ExecuteMaterial{NormalizedMaterial: &chainloop.NormalizedMaterial{Type: "SBOM_CYCLONEDX_JSON"}, Content: []byte("content")}

	testCases := []struct {
		name   string
		opts   *sdk.ExecutionRequest
		errMsg string
	}{
		{name: "invalid - missing input", errMsg: "invalid input"},
		{name: "invalid - missing input", opts: &sdk.ExecutionRequest{Input: &sdk.ExecuteInput{}}, errMsg: "invalid input"},
		{
			name: "invalid - missing material",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Material: &sdk.ExecuteMaterial{}},
			},
			errMsg: "invalid input",
		},
		{
			name: "invalid - invalid material",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Material: &sdk.ExecuteMaterial{NormalizedMaterial: &chainloop.NormalizedMaterial{Type: "invalid"}, Content: []byte("content")}},
			},
			errMsg: "invalid input type",
		},
		{
			name: "invalid - missing configuration",
			opts: &sdk.ExecutionRequest{
				Input: &sdk.ExecuteInput{Material: validMaterial},
			},
			errMsg: "missing registration configuration",
		},
		{
			name: "invalid - missing attachment configuration",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Material: validMaterial},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config"), Credentials: &sdk.Credentials{Password: "password"}},
			},
			errMsg: "missing attachment configuration",
		},
		{
			name: "invalid - missing registration configuration",
			opts: &sdk.ExecutionRequest{
				Input:          &sdk.ExecuteInput{Material: validMaterial},
				AttachmentInfo: &sdk.AttachmentResponse{},
			},
			errMsg: "missing registration configuration",
		},
		{
			name: "invalid - missing credentials",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Material: validMaterial},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config")},
				AttachmentInfo:   &sdk.AttachmentResponse{Configuration: []byte("config")},
			},
			errMsg: "missing credentials",
		},
		{
			name: "ok - all good",
			opts: &sdk.ExecutionRequest{
				Input:            &sdk.ExecuteInput{Material: validMaterial},
				RegistrationInfo: &sdk.RegistrationResponse{Configuration: []byte("config"), Credentials: &sdk.Credentials{Password: "password"}},
				AttachmentInfo:   &sdk.AttachmentResponse{Configuration: []byte("config")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExecuteOpts(tc.opts)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
