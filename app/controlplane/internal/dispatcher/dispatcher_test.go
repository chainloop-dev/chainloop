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

func (s *dispatcherTestSuite) TestCalculateDispatchQueue() {
	integrationInfoBuilder := func(b sdk.FanOut) *integrationInfo {
		return &integrationInfo{
			backend:            b,
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
			q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), tc.orgID, tc.workflowID)
			assert.Error(s.T(), err)
			assert.Nil(s.T(), q)
		})
	}

	s.T().Run("integration does NOT have integrations", func(t *testing.T) {
		q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), s.org.ID, s.emptyWorkflow.ID.String())
		require.NoError(t, err)
		require.NotNil(t, q)
		assert.Equal(t, make(materialsDispatch), q.materials)
		assert.Equal(t, make(attestationDispatch, 0), q.attestations)
	})

	s.T().Run("integration does have attestation-based integrations", func(t *testing.T) {
		wantAttestations := attestationDispatch{integrationInfoBuilder(s.ociIntegrationBackend)}

		q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), s.org.ID, s.workflow.ID.String())
		require.NoError(t, err)

		// Attestation integrations
		assert.Len(t, q.attestations, 1)
		assert.Equal(t, wantAttestations[0].backend, q.attestations[0].backend)
		assert.Equal(t, q.attestations[0].backend.Describe().ID, "OCI_INTEGRATION")
		assert.Equal(t, wantAttestations[0].attachmentConfig, q.attestations[0].attachmentConfig)
	})

	s.T().Run("integration does have material-based integrations", func(t *testing.T) {
		wantMaterials := make(materialsDispatch)
		wantMaterials[v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON] = []*integrationInfo{
			integrationInfoBuilder(s.cdxIntegrationBackend),
		}
		q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), s.org.ID, s.workflow.ID.String())
		require.NoError(t, err)

		// the map has two keys
		require.Len(t, q.materials, 2)
		sbomQueue := q.materials[v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON]
		// There are two integrations for SBOM material attached
		require.Len(t, sbomQueue, 2)
		assert.Equal(t, s.cdxIntegrationBackend, sbomQueue[0].backend)
		assert.Equal(t, "SBOM_INTEGRATION", sbomQueue[0].backend.Describe().ID)
		assert.Equal(t, s.cdxIntegrationBackend, sbomQueue[1].backend)
		assert.Equal(t, "SBOM_INTEGRATION", sbomQueue[1].backend.Describe().ID)
		assert.Equal(t, []byte("deadbeef"), sbomQueue[0].attachmentConfig)
		assert.Equal(t, []byte("deadbeef"), sbomQueue[1].attachmentConfig)

		// and one for any material type
		anyQueue := q.materials[v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED]
		require.Len(t, anyQueue, 1)
		assert.Equal(t, s.anyIntegrationBackend, anyQueue[0].backend)
		assert.Equal(t, "ANY_INTEGRATION", anyQueue[0].backend.Describe().ID)
		assert.Equal(t, []byte("deadbeef"), anyQueue[0].attachmentConfig)
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

	customImplementation := mockedSDK.NewFanOutExtension(s.T())
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

	s.cdxIntegrationBackend = &mockedIntegration{FanOutExtension: customImplementation, FanOutIntegration: b}
	s.cdxIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.cdxIntegrationBackend, config)
	require.NoError(s.T(), err)

	// Any material integration
	b, err = sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "ANY_INTEGRATION",
			Version:     "1.0",
			InputSchema: fanOutSchemas,
		},
		sdk.WithInputMaterial(v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED),
	)
	require.NoError(s.T(), err)

	s.anyIntegrationBackend = &mockedIntegration{FanOutExtension: customImplementation, FanOutIntegration: b}
	s.anyIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.anyIntegrationBackend, config)
	require.NoError(s.T(), err)

	// Attestation integration
	b, err = sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "OCI_INTEGRATION",
			Version:     "1.0",
			InputSchema: fanOutSchemas,
		},
		sdk.WithEnvelope(),
	)
	require.NoError(s.T(), err)

	s.ociIntegrationBackend = &mockedIntegration{FanOutExtension: customImplementation, FanOutIntegration: b}
	s.ociIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, "", s.ociIntegrationBackend, config)
	require.NoError(s.T(), err)

	// Attach all the integrations to the workflow
	for _, i := range []struct {
		integrationID string
		fanout        sdk.FanOut
	}{
		// We attach the CDX integration twice
		{s.cdxIntegration.ID.String(), s.cdxIntegrationBackend},
		{s.cdxIntegration.ID.String(), s.cdxIntegrationBackend},
		{s.anyIntegration.ID.String(), s.anyIntegrationBackend},
		{s.ociIntegration.ID.String(), s.ociIntegrationBackend},
	} {
		_, err = s.Integration.AttachToWorkflow(ctx, &biz.AttachOpts{
			OrgID:             s.org.ID,
			IntegrationID:     i.integrationID,
			WorkflowID:        s.workflow.ID.String(),
			FanOutIntegration: i.fanout,
			AttachmentConfig:  config,
		})

		require.NoError(s.T(), err)
	}

	// Register the integrations in the dispatcher
	registeredIntegrations := sdk.AvailableExtensions{s.cdxIntegrationBackend, s.anyIntegrationBackend, s.ociIntegrationBackend}
	s.dispatcher = New(s.Integration, nil, nil, mocks.NewCASClient(s.T()), registeredIntegrations, s.L)
}

type mockedIntegration struct {
	*sdk.FanOutIntegration
	*mockedSDK.FanOutExtension
}

// Utility struct to hold the test suite
type dispatcherTestSuite struct {
	suite.Suite
	testhelpers.UseCasesEachTestSuite
	cdxIntegration, ociIntegration, anyIntegration                      *biz.Integration
	cdxIntegrationBackend, ociIntegrationBackend, anyIntegrationBackend sdk.FanOut
	org, emptyOrg                                                       *biz.Organization
	workflow, emptyWorkflow                                             *biz.Workflow
	dispatcher                                                          *FanOutDispatcher
}
