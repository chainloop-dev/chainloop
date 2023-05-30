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

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

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

type PreRegistration struct {
	// Credentials to be persisted
	// JSON serializable
	Credentials *Credentials
	// Configuration to be persisted
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
}

type Credentials struct {
	URL, Username, Password string
}
