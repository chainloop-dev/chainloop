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
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	mockedSDK "github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"
)

func (s *dispatcherTestSuite) TestCalculateDispatchQueue() {
	bundledConfig := &sdk.BundledConfig{
		Registration: s.configAnyRegistration,
		Attachment:   s.configAnyAttachment,
		WorkflowID:   s.workflow.ID.String(),
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
		wantAttestations := attestationDispatch{
			&integrationInfo{
				backend: s.ociIntegrationBackend,
				config:  bundledConfig,
			}}

		q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), s.org.ID, s.workflow.ID.String())
		require.NoError(t, err)

		// Attestation integrations
		assert.Len(t, q.attestations, 1)
		assert.Equal(t, wantAttestations[0].backend, q.attestations[0].backend)
		assert.Equal(t, q.attestations[0].backend.Describe().ID, "OCI_INTEGRATION")
		assert.True(t, cmp.Equal(wantAttestations[0].config, q.attestations[0].config, protocmp.Transform()))
	})

	s.T().Run("integration does have material-based integrations", func(t *testing.T) {
		wantMaterials := make(materialsDispatch)
		wantMaterials[v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON] = []*integrationInfo{
			{
				backend: s.cdxIntegrationBackend,
				config:  bundledConfig,
			},
		}
		q, err := s.dispatcher.calculateDispatchQueue(context.TODO(), s.org.ID, s.workflow.ID.String())
		require.NoError(t, err)

		// the map has two keys
		require.Len(t, q.materials, 2)
		sbomQueue := q.materials[v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON]
		// There are two integrations for SBOM material attached
		require.Len(t, sbomQueue, 2)
		assert.Equal(t, s.cdxIntegrationBackend, sbomQueue[0].backend)
		assert.Equal(t, sbomQueue[0].backend.Describe().ID, "SBOM_INTEGRATION")
		assert.Equal(t, s.cdxIntegrationBackend, sbomQueue[1].backend)
		assert.Equal(t, sbomQueue[1].backend.Describe().ID, "SBOM_INTEGRATION")
		assert.True(t, cmp.Equal(bundledConfig, sbomQueue[0].config, protocmp.Transform()))
		assert.True(t, cmp.Equal(bundledConfig, sbomQueue[1].config, protocmp.Transform()))

		// and one for any material type
		anyQueue := q.materials[v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED]
		require.Len(t, anyQueue, 1)
		assert.Equal(t, s.anyIntegrationBackend, anyQueue[0].backend)
		assert.Equal(t, anyQueue[0].backend.Describe().ID, "ANY_INTEGRATION")
		assert.True(t, cmp.Equal(bundledConfig, anyQueue[0].config, protocmp.Transform()))
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

	integrationConfigRegistration, err := structpb.NewValue(map[string]interface{}{"firstName": "John"})
	assert.NoError(s.T(), err)
	integrationConfigAttachment, err := structpb.NewValue(map[string]interface{}{"attachment": "true"})
	assert.NoError(s.T(), err)
	customImplementation := mockedSDK.NewCustom(s.T())
	customImplementation.On("PreRegister", ctx, mock.Anything).Return(&sdk.PreRegistration{Configuration: integrationConfigRegistration}, nil)
	customImplementation.On("PreAttach", ctx, mock.Anything).Return(&sdk.PreAttachment{Configuration: integrationConfigAttachment}, nil)

	b, err := sdk.NewBaseIntegration("SBOM_INTEGRATION", "1.0", "test integration", sdk.WithInputMaterial(v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON))
	require.NoError(s.T(), err)

	s.cdxIntegrationBackend = &mockedIntegration{Custom: customImplementation, BaseIntegration: b}
	s.cdxIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, s.cdxIntegrationBackend, nil)
	require.NoError(s.T(), err)

	// Any material integration
	b, err = sdk.NewBaseIntegration("ANY_INTEGRATION", "1.0", "test integration", sdk.WithInputMaterial(v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED))
	require.NoError(s.T(), err)

	s.anyIntegrationBackend = &mockedIntegration{Custom: customImplementation, BaseIntegration: b}
	s.anyIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, s.anyIntegrationBackend, nil)
	require.NoError(s.T(), err)

	// Attestation integration
	b, err = sdk.NewBaseIntegration("OCI_INTEGRATION", "1.0", "test integration", sdk.WithEnvelope())
	require.NoError(s.T(), err)

	s.configAnyRegistration, err = anypb.New(integrationConfigRegistration)
	require.NoError(s.T(), err)

	s.configAnyAttachment, err = anypb.New(integrationConfigAttachment)
	require.NoError(s.T(), err)

	s.ociIntegrationBackend = &mockedIntegration{Custom: customImplementation, BaseIntegration: b}
	s.ociIntegration, err = s.Integration.RegisterAndSave(ctx, s.org.ID, s.ociIntegrationBackend, nil)
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
			AttachmentConfig:  s.configAnyAttachment,
		})

		require.NoError(s.T(), err)
	}

	// Register the integrations in the dispatcher
	registeredIntegrations := sdk.Initialized{s.cdxIntegrationBackend, s.anyIntegrationBackend, s.ociIntegrationBackend}
	s.dispatcher = New(s.Integration, nil, mocks.NewCASClient(s.T()), registeredIntegrations, s.L)
}

type mockedIntegration struct {
	*sdk.BaseIntegration
	*mockedSDK.Custom
}

// Utility struct to hold the test suite
type dispatcherTestSuite struct {
	suite.Suite
	testhelpers.UseCasesEachTestSuite
	cdxIntegration, ociIntegration, anyIntegration                      *biz.Integration
	cdxIntegrationBackend, ociIntegrationBackend, anyIntegrationBackend sdk.FanOut
	org, emptyOrg                                                       *biz.Organization
	workflow, emptyWorkflow                                             *biz.Workflow
	dispatcher                                                          *Dispatcher
	configAnyRegistration                                               *anypb.Any
	configAnyAttachment                                                 *anypb.Any
}
