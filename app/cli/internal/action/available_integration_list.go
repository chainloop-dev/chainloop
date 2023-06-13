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

package action

import (
	"context"
	"errors"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type AvailableIntegrationList struct {
	cfg *ActionsOpts
}

type AvailableIntegrationItem struct {
	ID                     string `json:"id"`
	Version                string `json:"version"`
	RegistrationJSONSchema string `json:"registrationJSONschema"`
	AttachmentJSONSchema   string `json:"attachmentJSONschema"`
}

func NewAvailableIntegrationList(cfg *ActionsOpts) *AvailableIntegrationList {
	return &AvailableIntegrationList{cfg}
}

func (action *AvailableIntegrationList) Run() ([]*AvailableIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	resp, err := client.ListAvailable(context.Background(), &pb.IntegrationsServiceListAvailableRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*AvailableIntegrationItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		i, err := pbAvailableIntegrationItemToAction(p)
		if err != nil {
			return nil, err
		}

		result = append(result, i)
	}

	return result, nil
}

func pbAvailableIntegrationItemToAction(in *pb.IntegrationsServiceListAvailableResponse_Integration) (*AvailableIntegrationItem, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}

	i := &AvailableIntegrationItem{
		ID: in.GetId(), Version: in.GetVersion(),
		RegistrationJSONSchema: string(in.GetRegistrationSchema()),
		AttachmentJSONSchema:   string(in.GetAttachmentSchema()),
	}

	return i, nil
}
