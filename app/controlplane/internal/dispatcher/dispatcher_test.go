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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	mockedSDK "github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/structpb"
)

var integrationInfoBuilder = func(b sdk.FanOut) *dispatchItem {
	return &dispatchItem{
		plugin:             b,
		registrationConfig: []byte("deadbeef"),
		attachmentConfig:   []byte("deadbeef"),
		credentials:        nil,
	}
}

func (s *dispatcherTestSuite) TestLoadInputsEnvelope() {
	queue := dispatchQueue{integrationInfoBuilder(s.ociIntegrationBackend)}
	envelope, err := testEnvelope("testdata/attestation.json")
	require.NoError(s.T(), err)

	err = s.dispatcher.loadInputs(context.TODO(), queue, envelope, "secret-name")
	assert.NoError(s.T(), err)

	// Only one integration is registered
	require.Len(s.T(), queue, 1)

	// Check that the integration is the OCI one
	dispatchItem := queue[0]
	assert.Equal(s.T(), s.ociIntegrationBackend, dispatchItem.plugin)

	got := dispatchItem.attestation
	require.NotNil(s.T(), got)

	// It contains the envelope and its hash
	assert.Equal(s.T(), envelope, got.Envelope)
	assert.Equal(s.T(), "33683275ee73f7f019d57b7522dfdfa1eb737b6a7c61e9c4dc2a03a48ef6e1ef", got.Hash.Hex)

	// And the statement and predicate
	assert.NotNil(s.T(), got.Statement)
	assert.NotNil(s.T(), got.Predicate)
	// And it contains the actual information from the envelope
	assert.Len(s.T(), got.Predicate.GetMaterials(), 3)
}

func (s *dispatcherTestSuite) TestLoadInputsMaterials() {
	queue := dispatchQueue{
		integrationInfoBuilder(s.ociIntegrationBackend),
		integrationInfoBuilder(s.containerIntegrationBackend),
		integrationInfoBuilder(s.cdxIntegrationBackend),
	}

	envelope, err := testEnvelope("testdata/attestation.json")
	require.NoError(s.T(), err)

	// Simulate SBOM download
	s.casClient.On("Download", mock.Anything, "secret-name", mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		buf := bytes.NewBuffer([]byte("SBOM Content"))
		_, err := io.Copy(args.Get(2).(io.Writer), buf)
		s.NoError(err)
	})

	err = s.dispatcher.loadInputs(context.TODO(), queue, envelope, "secret-name")
	assert.NoError(s.T(), err)
	require.Len(s.T(), queue, 3)

	// OCI integration has no materials
	assert.Equal(s.T(), "OCI_INTEGRATION", queue[0].plugin.Describe().ID)
	assert.Len(s.T(), queue[0].materials, 0)

	// Container integration has container image information
	dispathItem, materials := queue[1], queue[1].materials
	assert.Equal(s.T(), "CONTAINER_INTEGRATION", dispathItem.plugin.Describe().ID)
	assert.Len(s.T(), materials, 1)
	assert.Equal(s.T(), "image", materials[0].Name)
	assert.Equal(s.T(), "sha256:264f55a6ff9cec2f4742a9faacc033b29f65c04dd4480e71e23579d484288d61", materials[0].Hash.String())
	assert.Equal(s.T(), "index.docker.io/bitnami/nginx", string(materials[0].Content))

	// Dependency-Track integration has two sboms with the content already downloaded
	dispathItem, materials = queue[2], queue[2].materials
	assert.Equal(s.T(), "SBOM_INTEGRATION", dispathItem.plugin.Describe().ID)

	require.Len(s.T(), materials, 2)
	assert.Equal(s.T(), "skynet-sbom", materials[0].Name)
	assert.Equal(s.T(), "SBOM Content", string(materials[0].Content))

	assert.Equal(s.T(), "skynet2-sbom", materials[1].Name)
	assert.Equal(s.T(), "SBOM Content", string(materials[1].Content))
}

func testEnvelope(filePath string) (*dsse.Envelope, error) {
	var envelope dsse.Envelope
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &envelope)
	if err != nil {
		return nil, err
	}

	return &envelope, nil
}

func (s *dispatcherTestSuite) TestInitDispatchQueue() {
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

	s.T().Run("workflow does NOT have integrations", func(t *testing.T) {
		q, err := s.dispatcher.initDispatchQueue(context.TODO(), s.org.ID, s.emptyWorkflow.ID.String())
		require.NoError(t, err)
		require.NotNil(t, q)
		assert.Len(t, q, 0)
	})

	s.T().Run("workflow does have integrations", func(t *testing.T) {
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
	registeredIntegrations := sdk.AvailablePlugins{
		&sdk.FanOutP{FanOut: s.cdxIntegrationBackend},
		&sdk.FanOutP{FanOut: s.containerIntegrationBackend},
		&sdk.FanOutP{FanOut: s.ociIntegrationBackend},
	}
	l := log.NewStdLogger(io.Discard)

	s.casClient = mocks.NewCASClient(s.T())
	s.dispatcher = New(s.Integration, nil, nil, s.casClient, registeredIntegrations, l)
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
	casClient                                                                 *mocks.CASClient
}
