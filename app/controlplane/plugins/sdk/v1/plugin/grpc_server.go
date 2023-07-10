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

package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/plugin/api"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type fanOutGRPCServer struct {
	impl sdk.FanOut
	api.UnimplementedFanoutServiceServer
}

func (b *fanOutGRPCServer) Describe(_ context.Context, _ *api.DescribeRequest) (*api.DescribeResponse, error) {
	info := b.impl.Describe()

	return api.DescribeSDKToProto(info)
}

func (b *fanOutGRPCServer) ValidateRegistration(_ context.Context, req *api.ValidateRegistrationRequest) (*api.ValidateRegistrationResponse, error) {
	err := b.impl.ValidateRegistrationRequest(req.JsonPayload)

	resp := &api.ValidateRegistrationResponse{Valid: err == nil}

	if err != nil {
		resp.Error = err.Error()
	}

	return resp, err
}

func (b *fanOutGRPCServer) ValidateAttachment(_ context.Context, req *api.ValidateAttachmentRequest) (*api.ValidateAttachmentResponse, error) {
	err := b.impl.ValidateAttachmentRequest(req.JsonPayload)

	resp := &api.ValidateAttachmentResponse{Valid: err == nil}

	if err != nil {
		resp.Error = err.Error()
	}

	return resp, err
}

func (b *fanOutGRPCServer) IsSubscribedTo(_ context.Context, req *api.IsSubscribedToRequest) (*api.IsSubscribedToResponse, error) {
	return &api.IsSubscribedToResponse{
		Subscribed: b.impl.IsSubscribedTo(req.MaterialType),
	}, nil
}

func (b *fanOutGRPCServer) String(_ context.Context, _ *api.StringRequest) (*api.StringResponse, error) {
	return &api.StringResponse{
		Value: b.impl.String(),
	}, nil
}

func (b *fanOutGRPCServer) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	resp, err := b.impl.Register(ctx, &sdk.RegistrationRequest{
		Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}

	return api.RegistrationSDKToProto(resp)
}

func (b *fanOutGRPCServer) Attach(ctx context.Context, req *api.AttachRequest) (*api.AttachResponse, error) {
	registrationInfo, err := api.RegistrationProtoToSDK(req.RegistrationInfo)
	if err != nil {
		return nil, err
	}

	resp, err := b.impl.Attach(ctx, &sdk.AttachmentRequest{
		Payload:          req.Payload,
		RegistrationInfo: registrationInfo,
	})
	if err != nil {
		return nil, err
	}

	return api.AttachSDKToProto(resp)
}

func (b *fanOutGRPCServer) Execute(ctx context.Context, req *api.ExecuteRequest) (*api.ExecuteResponse, error) {
	registrationInfo, err := api.RegistrationProtoToSDK(req.RegistrationInfo)
	if err != nil {
		return nil, err
	}

	attachmentInfo, err := api.AttachProtoToSDK(req.AttachmentInfo)
	if err != nil {
		return nil, err
	}

	// Generate attestation info from envelope
	attestationInput, err := envelopeToAttestationInfo(req.Envelope)
	if err != nil {
		return nil, fmt.Errorf("converting envelope to attestation info: %w", err)
	}

	opts := &sdk.ExecutionRequest{
		RegistrationInfo:  registrationInfo,
		AttachmentInfo:    attachmentInfo,
		ChainloopMetadata: api.MetadataProtoToSDK(req.Metadata),
		Input: &sdk.ExecuteInput{
			Attestation: attestationInput,
			Materials:   make([]*sdk.ExecuteMaterial, 0),
		},
	}

	for _, material := range req.Materials {
		opts.Input.Materials = append(opts.Input.Materials, api.MaterialProtoToSDK(material))
	}

	return &api.ExecuteResponse{}, b.impl.Execute(ctx, opts)
}

func envelopeToAttestationInfo(envelopeJSON []byte) (*sdk.ExecuteAttestation, error) {
	var envelope *dsse.Envelope
	if err := json.Unmarshal(envelopeJSON, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshalling envelope: %w", err)
	}

	// get the in_toto statement from the envelope if present
	statement, err := chainloop.ExtractStatement(envelope)
	if err != nil {
		return nil, fmt.Errorf("extracting statement: %w", err)
	}

	// Iterate over the materials in the attestation and dispatch them to the integrations that are subscribed to them
	predicate, err := chainloop.ExtractPredicate(envelope)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	return &sdk.ExecuteAttestation{
		Envelope:  envelope,
		Statement: statement,
		Predicate: predicate,
	}, nil
}
