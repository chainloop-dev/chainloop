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
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestValidateConfiguration(t *testing.T) {
	testIntegrationConfig := func(allowAutoCreate bool) *v1.RegistrationConfig {
		return &v1.RegistrationConfig{
			AllowAutoCreate: allowAutoCreate,
			Domain:          "domain",
		}
	}

	testAttachmentConfig := func(projectID, projectName string) *v1.AttachmentConfig {
		if projectID != "" {
			return &v1.AttachmentConfig{
				Project: &v1.AttachmentConfig_ProjectId{ProjectId: projectID},
			}
		} else if projectName != "" {
			return &v1.AttachmentConfig{
				Project: &v1.AttachmentConfig_ProjectName{ProjectName: projectName},
			}
		}

		return &v1.AttachmentConfig{}
	}

	tests := []struct {
		integrationConfig *v1.RegistrationConfig
		attachmentConfig  *v1.AttachmentConfig
		errorMsg          string
	}{
		{nil, nil, "invalid configuration"},
		// autocreate required but not supported
		{testIntegrationConfig(false), testAttachmentConfig("", "new-project"), "auto creation of projects is not supported in this integration"},
		// autocreate required and supported
		{testIntegrationConfig(true), testAttachmentConfig("", "new-project"), ""},
		// Neither projectID nor autocreate provided
		{testIntegrationConfig(false), testAttachmentConfig("", ""), "AttachmentConfig.Project: value is required"},
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
		ID:          "dependencytrack.cyclonedx.v1",
		Description: "Dependency Track CycloneDX Software Bill Of Materials Integration",
		Version:     "1.0",
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
	config, _ := anypb.New(&emptypb.Empty{})

	testCases := []struct {
		name   string
		opts   *sdk.ExecuteReq
		errMsg string
	}{
		{name: "invalid - missing input", errMsg: "invalid input"},
		{name: "invalid - missing input", opts: &sdk.ExecuteReq{Input: &sdk.ExecuteInput{}}, errMsg: "invalid input"},
		{
			name: "invalid - missing material",
			opts: &sdk.ExecuteReq{
				Input: &sdk.ExecuteInput{Material: &sdk.ExecuteMaterial{}},
			},
			errMsg: "invalid input",
		},
		{
			name: "invalid - invalid material",
			opts: &sdk.ExecuteReq{
				Input: &sdk.ExecuteInput{Material: &sdk.ExecuteMaterial{NormalizedMaterial: &chainloop.NormalizedMaterial{Type: "invalid"}, Content: []byte("content")}},
			},
			errMsg: "invalid input type",
		},
		{
			name: "invalid - missing configuration",
			opts: &sdk.ExecuteReq{
				Input: &sdk.ExecuteInput{Material: validMaterial},
			},
			errMsg: "missing configuration",
		},
		{
			name: "invalid - missing attachment configuration",
			opts: &sdk.ExecuteReq{
				Input:  &sdk.ExecuteInput{Material: validMaterial},
				Config: &sdk.BundledConfig{Registration: config},
			},
			errMsg: "missing configuration",
		},
		{
			name: "invalid - missing registration configuration",
			opts: &sdk.ExecuteReq{
				Input:  &sdk.ExecuteInput{Material: validMaterial},
				Config: &sdk.BundledConfig{Attachment: config},
			},
			errMsg: "missing configuration",
		},
		{
			name: "invalid - missing credentials",
			opts: &sdk.ExecuteReq{
				Input:  &sdk.ExecuteInput{Material: validMaterial},
				Config: &sdk.BundledConfig{Registration: config, Attachment: config},
			},
			errMsg: "missing credentials",
		},
		{
			name: "ok - all good",
			opts: &sdk.ExecuteReq{
				Input:  &sdk.ExecuteInput{Material: validMaterial},
				Config: &sdk.BundledConfig{Registration: config, Attachment: config, Credentials: &sdk.Credentials{Password: "password"}},
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
