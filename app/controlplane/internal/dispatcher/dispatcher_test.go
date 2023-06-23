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

package dispatcher

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	mockedSDK "github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *dispatcherTestSuite) TestInitDispatchQueue() {
	integrationInfoBuilder := func(b sdk.FanOut) *dispatchItem {
		return &dispatchItem{
			plugin:             b,
			registrationConfig: []byte("deadbeef"),
			attachmentConfig:   []byte("deadbeef"),
			credentials:        nil,
		}
	}

	testCasesWithError := []struct {
		name       string
		orgID      string
		workflowID string
		want       *dispatchQueue
	}{
		{
			name:       "workflow not found",
			orgID:      s.org.ID,
			workflowID: "deadbeef",
		},
		{
			name:       "workflow in another org",
			orgID:      s.emptyOrg.ID,
			workflowID: s.workflow.ID.String(),
		},
	}

	for _, tc := range testCasesWithError {
		s.Run(tc.name, func() {
			q, err := s.dispatcher.initDispatchQueue(context.TODO(), tc.orgID, tc.workflowID)
			assert.Error(s.T(), err)
			assert.Nil(s.T(), q)
		})
	}

	s.T().Run("integration does NOT have integrations", func(t *testing.T) {
		q, err := s.dispatcher.initDispatchQueue(context.TODO(), s.org.ID, s.emptyWorkflow.ID.String())
		require.NoError(t, err)
		require.NotNil(t, q)
		assert.Len(t, q, 0)
	})

	s.T().Run("integration does have integrations", func(t *testing.T) {
		wantAttestations := dispatchQueue{
			integrationInfoBuilder(s.ociIntegrationBackend), integrationInfoBuilder(s.containerIntegrationBackend),
			integrationInfoBuilder(s.cdxIntegrationBackend), integrationInfoBuilder(s.cdxIntegrationBackend),
		}

		q, err := s.dispatcher.initDispatchQueue(context.TODO(), s.org.ID, s.workflow.ID.String())
		require.NoError(t, err)

		// There are 4 integrations attached
		require.Len(t, q, 4)

		for i, tc := range []struct{ id, subscribedMaterial string }{
			{"OCI_INTEGRATION", ""},
			{"CONTAINER_INTEGRATION", "CONTAINER_IMAGE"},
			{"SBOM_INTEGRATION", "SBOM_CYCLONEDX_JSON"},
			{"SBOM_INTEGRATION", "SBOM_CYCLONEDX_JSON"},
		} {
			assert.Equal(t, tc.id, q[i].plugin.Describe().ID)
			assert.Equal(t, wantAttestations[i].plugin, q[i].plugin)
			assert.Equal(t, wantAttestations[i].attachmentConfig, q[i].attachmentConfig)

			if tc.subscribedMaterial != "" {
				assert.True(t, q[i].plugin.IsSubscribedTo(tc.subscribedMaterial))
			}
		}
	})
}

func TestDispatcher(t *testing.T) {
	suite.Run(t, new(dispatcherTestSuite))
}

func (s *dispatcherTestSuite) SetupTest() {
	// Register three integrations
	// SBOM material integration
	ctx := context.Background()
	var err error

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	// Create org, integration and oci repository
	s.org, err = s.Organization.Create(ctx, "testing org")
	assert.NoError(s.T(), err)

	// Create org, integration and oci repository
	s.emptyOrg, err = s.Organization.Create(ctx, "empty org")
	assert.NoError(s.T(), err)

	// Workflow
	s.workflow, err = s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: s.org.ID})
	assert.NoError(s.T(), err)

	// Workflow
	s.emptyWorkflow, err = s.Workflow.Create(ctx, &biz.CreateOpts{Name: "empty workflow", OrgID: s.org.ID})
	assert.NoError(s.T(), err)

	customImplementation := mockedSDK.NewFanOutPlugin(s.T())
	customImplementation.On("Register", ctx, mock.Anything).Return(&sdk.RegistrationResponse{Configuration: []byte("deadbeef")}, nil)
	customImplementation.On("Attach", ctx, mock.Anything).Return(&sdk.AttachmentResponse{Configuration: []byte("deadbeef")}, nil)
	type schema struct {
		TestProperty string
	}

	fanOutSchemas := &sdk.InputSchema{Registration: schema{}, Attachment: schema{}}

	b, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "SBOM_INTEGRATION",
			Version:     "1.0",
			InputSchema: fanOutSchemas,
		},
		sdk.WithInputMaterial(v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
	)
	require.NoError(s.T(), err)

	// Registration configuration
	config, _ := structpb.NewStruct(map[string]interface{}{"TestProperty": "testValue"})

	s.cdxIntegrationBackend = &mockedIntegration{FanOutPlugin: customImplementation, FanOutIntegration: b}
	s.cdxIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.cdxIntegrationBackend, config)
	require.NoError(s.T(), err)

	b, err = sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "CONTAINER_INTEGRATION",
			Version:     "1.0",
			InputSchema: fanOutSchemas,
		},
		sdk.WithInputMaterial(v1.CraftingSchema_Material_CONTAINER_IMAGE),
	)
	require.NoError(s.T(), err)

	s.containerIntegrationBackend = &mockedIntegration{FanOutPlugin: customImplementation, FanOutIntegration: b}
	s.containerIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.containerIntegrationBackend, config)
	require.NoError(s.T(), err)

	// Attestation integration
	b, err = sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "OCI_INTEGRATION",
			Version:     "1.0",
			InputSchema: fanOutSchemas,
		},
	)
	require.NoError(s.T(), err)

	s.ociIntegrationBackend = &mockedIntegration{FanOutPlugin: customImplementation, FanOutIntegration: b}
	s.ociIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.ociIntegrationBackend, config)
	require.NoError(s.T(), err)

	// Attach all the integrations to the workflow
	for _, i := range []struct {
		integrationID string
		fanOut        sdk.FanOut
	}{
		// We attach the CDX integration twice
		{s.cdxIntegration.ID.String(), s.cdxIntegrationBackend},
		{s.cdxIntegration.ID.String(), s.cdxIntegrationBackend},
		{s.containerIntegration.ID.String(), s.containerIntegrationBackend},
		{s.ociIntegration.ID.String(), s.ociIntegrationBackend},
	} {
		_, err = s.Integration.AttachToWorkflow(ctx, &biz.AttachOpts{
			OrgID:             s.org.ID,
			IntegrationID:     i.integrationID,
			WorkflowID:        s.workflow.ID.String(),
			FanOutIntegration: i.fanOut,
			AttachmentConfig:  config,
		})

		require.NoError(s.T(), err)
	}

	// Register the integrations in the dispatcher
	registeredIntegrations := sdk.AvailablePlugins{s.cdxIntegrationBackend, s.containerIntegrationBackend, s.ociIntegrationBackend}
	s.dispatcher = New(s.Integration, nil, nil, mocks.NewCASClient(s.T()), registeredIntegrations, s.L)
}

type mockedIntegration struct {
	*sdk.FanOutIntegration
	*mockedSDK.FanOutPlugin
}

// Utility struct to hold the test suite
type dispatcherTestSuite struct {
	suite.Suite
	testhelpers.UseCasesEachTestSuite
	cdxIntegration, ociIntegration, containerIntegration                      *biz.Integration
	cdxIntegrationBackend, ociIntegrationBackend, containerIntegrationBackend sdk.FanOut
	org, emptyOrg                                                             *biz.Organization
	workflow, emptyWorkflow                                                   *biz.Workflow
	dispatcher                                                                *FanOutDispatcher
}
