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

package integrations

import (
	"context"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type InputType int64

type Inputs struct {
	DSSEnvelope   bool
	InputMaterial *InputMaterial
}

type InputMaterial struct {
	// Name of the material kind that the integration expects
	Type schemaapi.CraftingSchema_Material_MaterialType
}

type Integration struct {
	// Identifier of the integration
	ID string
	// Kind of inputs does the integration expect as part of the execution
	SubscribedInputs *Inputs
}

func (i *Integration) ExpectedInputs() *Inputs {
	return i.SubscribedInputs
}

// Registrable is the interface that needs to be implemented by all integrations
// To be able to be registered in Chainloop control plane
type Registrable interface {
	// Validate, marshall and return the configuration that needs to be persisted
	PreRegister(ctx context.Context, req *anypb.Any) (*PreRegistration, error)
}

// Attachable describes what an integration needs to implement to be able to get "attached" to a workflow
type Attachable interface {
	// Validate that the attachment configuration is valid in the context of the provided registration
	PreAttach(ctx context.Context, c *BundledConfig) (*PreAttachment, error)
}

// An execute method will receive either the envelope or a material as input
// The material will contain its content as well as the metadata
type ExecuteInput struct {
	DSSEnvelope *dsse.Envelope
	Material    *ExecuteMaterial
}

type ExecuteMaterial struct {
	*chainloop.NormalizedMaterial
	// Content of the material already downloaded
	Content []byte
}

type ExecuteOpts struct {
	Config *BundledConfig
	Input  *ExecuteInput
}

type Executable interface {
	// What kind of inputs does the integration expect
	ExpectedInputs() *Inputs
	Execute(ctx context.Context, opts *ExecuteOpts) error
}

type PreRegistration struct {
	// Credentials to be persisted in Credentials Manager
	// JSON serializable
	Credentials *Credentials
	// Configuration to be persisted in DB
	Configuration proto.Message
	// registration kind
	Kind string
}

type PreAttachment struct {
	// Configuration to be persisted
	Configuration proto.Message
}

// BundledConfig is the collection of the registration and attachment configuration
type BundledConfig struct {
	// Registration configuration
	Registration *anypb.Any
	// Attachment configuration
	Attachment *anypb.Any
	// Stored credentials
	Credentials *Credentials
	// Chainloop Metadata
	WorkflowID string
}

type Credentials struct {
	URL, Username, Password string
}
