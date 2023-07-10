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
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/plugin/api"
)

// This is a gRPC client implementation of the FanOut interface.
type fanOutGRPCClient struct {
	client api.FanoutServiceClient
}

var _ sdk.FanOut = (*fanOutGRPCClient)(nil)

func (c *fanOutGRPCClient) Describe() *sdk.IntegrationInfo {
	resp, err := c.client.Describe(context.Background(), &api.DescribeRequest{})
	if err != nil {
		return nil
	}

	info, err := api.DescribeProtoToSDK(resp)
	if err != nil {
		return nil
	}

	return info
}

func (c *fanOutGRPCClient) ValidateRegistrationRequest(payload []byte) error {
	resp, err := c.client.ValidateRegistration(context.Background(), &api.ValidateRegistrationRequest{
		JsonPayload: payload,
	})
	if err != nil {
		return api.ProtoErrToErr(err)
	}

	if !resp.Valid {
		return errors.New(resp.Error)
	}

	return nil
}

func (c *fanOutGRPCClient) ValidateAttachmentRequest(payload []byte) error {
	resp, err := c.client.ValidateAttachment(context.Background(), &api.ValidateAttachmentRequest{
		JsonPayload: payload,
	})
	if err != nil {
		return api.ProtoErrToErr(err)
	}

	if !resp.Valid {
		return errors.New(resp.Error)
	}

	return nil
}

func (c *fanOutGRPCClient) String() string {
	resp, err := c.client.String(context.Background(), &api.StringRequest{})
	if err != nil {
		return ""
	}

	return resp.Value
}

func (c *fanOutGRPCClient) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	resp, err := c.client.Register(ctx, &api.RegisterRequest{
		Payload: req.Payload,
	})

	if err != nil {
		return nil, api.ProtoErrToErr(err)
	}

	r, err := api.RegistrationProtoToSDK(resp)
	if err != nil {
		return nil, api.ProtoErrToErr(err)
	}

	return r, nil
}

func (c *fanOutGRPCClient) Attach(ctx context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	regResp, err := api.RegistrationSDKToProto(req.RegistrationInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to convert registration info: %w", err)
	}

	resp, err := c.client.Attach(ctx, &api.AttachRequest{
		Payload:          req.Payload,
		RegistrationInfo: regResp,
	})
	if err != nil {
		return nil, api.ProtoErrToErr(err)
	}

	return api.AttachProtoToSDK(resp)
}

func (c *fanOutGRPCClient) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	regResp, err := api.RegistrationSDKToProto(req.RegistrationInfo)
	if err != nil {
		return fmt.Errorf("failed to convert registration info: %w", err)
	}

	attResp, err := api.AttachSDKToProto(req.AttachmentInfo)
	if err != nil {
		return fmt.Errorf("failed to convert attachment info: %w", err)
	}

	// We send the envelope json encoded
	envelopeJSON, err := json.Marshal(req.Input.Attestation.Envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation envelope: %w", err)
	}

	reqPayload := &api.ExecuteRequest{
		RegistrationInfo: regResp,
		AttachmentInfo:   attResp,
		Envelope:         envelopeJSON,
		Metadata:         api.MetadataSDKToProto(req.ChainloopMetadata),
	}

	for _, m := range req.Input.Materials {
		reqPayload.Materials = append(reqPayload.Materials, api.MaterialSDKToProto(m))
	}

	_, err = c.client.Execute(ctx, reqPayload)
	return api.ProtoErrToErr(err)
}

func (c *fanOutGRPCClient) IsSubscribedTo(m string) bool {
	resp, err := c.client.IsSubscribedTo(context.Background(), &api.IsSubscribedToRequest{
		MaterialType: m,
	})
	if err != nil {
		return false
	}

	return resp.Subscribed
}
