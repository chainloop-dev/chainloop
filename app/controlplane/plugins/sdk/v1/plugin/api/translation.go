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

package api

import (
	"errors"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	status "google.golang.org/grpc/status"
)

func DescribeProtoToSDK(pd *DescribeResponse) (*sdk.IntegrationInfo, error) {
	info := &sdk.IntegrationInfo{
		ID:                     pd.Id,
		Version:                pd.Version,
		Description:            pd.Description,
		RegistrationJSONSchema: pd.RegistrationJsonSchema,
		AttachmentJSONSchema:   pd.AttachmentJsonSchema,
		SubscribedMaterials:    make([]*sdk.InputMaterial, 0),
	}

	for _, m := range pd.SubscribedMaterials {
		materialType, ok := schemaapi.CraftingSchema_Material_MaterialType_value[m]
		if !ok {
			return nil, fmt.Errorf("invalid material type %s", m)
		}

		info.SubscribedMaterials = append(info.SubscribedMaterials, &sdk.InputMaterial{Type: schemaapi.CraftingSchema_Material_MaterialType(materialType)})
	}

	return info, nil
}

func DescribeSDKToProto(in *sdk.IntegrationInfo) (*DescribeResponse, error) {
	var materials = make([]string, 0, len(in.SubscribedMaterials))
	for _, m := range in.SubscribedMaterials {
		materials = append(materials, m.Type.String())
	}

	return &DescribeResponse{
		Id:                     in.ID,
		Version:                in.Version,
		Description:            in.Description,
		RegistrationJsonSchema: in.RegistrationJSONSchema,
		AttachmentJsonSchema:   in.AttachmentJSONSchema,
		SubscribedMaterials:    materials,
	}, nil
}

func RegistrationSDKToProto(in *sdk.RegistrationResponse) (*RegisterResponse, error) {
	resp := &RegisterResponse{
		Configuration: in.Configuration,
	}

	if creds := in.Credentials; creds != nil {
		resp.Credentials = &RegisterResponse_Credentials{
			Username: creds.Username,
			Password: creds.Password,
			Url:      creds.URL,
		}
	}

	return resp, nil
}

func RegistrationProtoToSDK(in *RegisterResponse) (*sdk.RegistrationResponse, error) {
	resp := &sdk.RegistrationResponse{
		Configuration: in.Configuration,
	}

	if creds := in.Credentials; creds != nil {
		resp.Credentials = &sdk.Credentials{
			Username: creds.Username,
			Password: creds.Password,
			URL:      creds.Url,
		}
	}

	return resp, nil
}

func AttachSDKToProto(in *sdk.AttachmentResponse) (*AttachResponse, error) {
	return &AttachResponse{
		Configuration: in.Configuration,
	}, nil
}

func AttachProtoToSDK(in *AttachResponse) (*sdk.AttachmentResponse, error) {
	return &sdk.AttachmentResponse{
		Configuration: in.Configuration,
	}, nil
}

func MetadataSDKToProto(in *sdk.ChainloopMetadata) *ExecuteRequest_Metadata {
	return &ExecuteRequest_Metadata{
		WorkflowId:      in.WorkflowID,
		WorkflowName:    in.WorkflowName,
		WorkflowProject: in.WorkflowProject,
		WorkflowRunId:   in.WorkflowRunID,
	}
}

func MetadataProtoToSDK(in *ExecuteRequest_Metadata) *sdk.ChainloopMetadata {
	return &sdk.ChainloopMetadata{
		WorkflowID:      in.WorkflowId,
		WorkflowName:    in.WorkflowName,
		WorkflowProject: in.WorkflowProject,
		WorkflowRunID:   in.WorkflowRunId,
	}
}

func MaterialSDKToProto(in *sdk.ExecuteMaterial) *ExecuteRequest_NormalizedMaterial {
	return &ExecuteRequest_NormalizedMaterial{
		Content:       in.Content,
		Name:          in.Name,
		Type:          in.Type,
		Value:         in.Value,
		Hash:          in.Hash.String(),
		UploadedToCas: in.UploadedToCAS,
	}
}

func MaterialProtoToSDK(in *ExecuteRequest_NormalizedMaterial) *sdk.ExecuteMaterial {
	if in == nil {
		return nil
	}

	hash, _ := cr_v1.NewHash(in.Hash)

	return &sdk.ExecuteMaterial{
		Content: in.Content,
		NormalizedMaterial: &chainloop.NormalizedMaterial{
			Name:          in.Name,
			Type:          in.Type,
			Value:         in.Value,
			UploadedToCAS: in.UploadedToCas,
			Hash:          &hash,
		},
	}
}

func ProtoErrToErr(err error) error {
	if err == nil {
		return nil
	}

	s, ok := status.FromError(err)
	if !ok {
		return err
	}

	return errors.New(s.Message())
}
