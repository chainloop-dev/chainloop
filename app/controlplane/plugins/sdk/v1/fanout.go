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

package sdk

import (
	"context"
	"fmt"
	"io"
	"time"

	crv1 "github.com/google/go-containerregistry/pkg/v1"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const IntegrationKindFanOut = "fan-out"

// FanOut a FanOut is an integration for which we expect to fan out data to other systems
type FanOut interface {
	// Integration has to be implemented by all integrations
	Integration

	// GetSubscribedMaterials returns material types an integration expect as part of the execution
	GetSubscribedMaterials() []*InputMaterial
	// IsSubscribedTo Returns if the integration is subscribed to the material type
	IsSubscribedTo(materialType string) bool
}

// FanOutIntegration provides a base implementation to be embedded in FanOut plugins
type FanOutIntegration struct {
	*IntegrationBase

	// Material types an integration expect as part of the execution
	subscribedMaterials []*InputMaterial

	log    log.Logger
	Logger *log.Helper
}

type ChainloopMetadata struct {
	Workflow    *ChainloopMetadataWorkflow
	WorkflowRun *ChainloopMetadataWorkflowRun
}

type ChainloopMetadataWorkflowRun struct {
	ID                string
	State             string
	StartedAt         time.Time
	FinishedAt        time.Time
	RunnerType        string
	RunURL            string
	AttestationDigest string
}

type ChainloopMetadataWorkflow struct {
	ID, Name, Team, Project string
}

// FanOutPayload is the request to execute the FanOut integration
type FanOutPayload struct {
	*ChainloopMetadata

	// An execute method will receive either the envelope or a material as input
	// The material will contain its content as well as the metadata
	Attestation *ExecuteAttestation
	Materials   []*ExecuteMaterial
}

type ExecuteAttestation struct {
	Envelope *dsse.Envelope
	// Hash of the envelope
	Hash      crv1.Hash
	Statement *intoto.Statement
	Predicate chainloop.NormalizablePredicate
}

type ExecuteMaterial struct {
	*chainloop.NormalizedMaterial
	// Content of the material already downloaded
	Content []byte
}

type InputMaterial struct {
	// Name of the material kind that the integration expects
	Type schemaapi.CraftingSchema_Material_MaterialType
}

type NewOpt func(*FanOutIntegration)

func WithInputMaterial(materialType schemaapi.CraftingSchema_Material_MaterialType) NewOpt {
	return func(c *FanOutIntegration) {
		material := &InputMaterial{Type: materialType}

		switch {
		case len(c.subscribedMaterials) == 0: // Materials struct is empty
			c.subscribedMaterials = []*InputMaterial{material}
		default: // Materials struct contains data
			c.subscribedMaterials = append(c.subscribedMaterials, material)
		}
	}
}

type NewParams struct {
	ID, Name, Version, Description string
	Logger                         log.Logger
	InputSchema                    *InputSchema
}

func NewFanOut(p *NewParams, opts ...NewOpt) (*FanOutIntegration, error) {
	base, err := NewIntegrationBase(p.ID, p.Name, p.Version, p.Description, IntegrationKindFanOut, p.InputSchema)
	if err != nil {
		return nil, err
	}

	c := &FanOutIntegration{
		IntegrationBase:     base,
		log:                 p.Logger,
		subscribedMaterials: []*InputMaterial{},
	}

	if c.log == nil {
		c.log = log.NewStdLogger(io.Discard)
	}

	c.Logger = servicelogger.ScopedHelper(c.log, fmt.Sprintf("plugins/%s", p.ID))

	for _, opt := range opts {
		opt(c)
	}

	if err := validateInputs(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (i *FanOutIntegration) GetSubscribedMaterials() []*InputMaterial {
	return i.subscribedMaterials
}

func (i *FanOutIntegration) IsSubscribedTo(m string) bool {
	if i.subscribedMaterials == nil {
		return false
	}

	for _, material := range i.subscribedMaterials {
		if material.Type.String() == m {
			return true
		}
	}

	return false
}

func (i *FanOutIntegration) String() string {
	inputs := i.subscribedMaterials

	subscribedMaterials := make([]string, len(inputs))
	for i, m := range inputs {
		subscribedMaterials[i] = m.Type.String()
	}

	return fmt.Sprintf("id=%s, version=%s, expectedMaterials=%s", i.Id, i.Version, subscribedMaterials)
}

func validateInputs(c *FanOutIntegration) error {
	for _, m := range c.subscribedMaterials {
		if m.Type == schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED {
			return fmt.Errorf("%s is not a valid material type", m.Type)
		}
	}

	return nil
}

// Methods to be implemented by the specific integration

func (i *FanOutIntegration) Register(_ context.Context, _ *RegistrationRequest) (*RegistrationResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *FanOutIntegration) Attach(_ context.Context, _ *AttachmentRequest) (*AttachmentResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *FanOutIntegration) Execute(_ context.Context, _ *ExecutionRequest) error {
	return fmt.Errorf("not implemented")
}

// List of loaded integrations
type AvailablePlugins []*FanOutP
type FanOutP struct {
	FanOut
	DisposeFunc func()
}

type FanOutFactory = func(l log.Logger) (FanOut, error)

// FindByID returns the integration with the given ID from the list of available integrations
// If not found, an error is returned
func (i AvailablePlugins) FindByID(id string) (FanOut, error) {
	for _, integration := range i {
		if integration.Describe().ID == id {
			return integration, nil
		}
	}

	return nil, fmt.Errorf("integration %q not found", id)
}

func (i AvailablePlugins) Cleanup() {
	for _, plugin := range i {
		if plugin.DisposeFunc != nil {
			plugin.DisposeFunc()
		}
	}
}
