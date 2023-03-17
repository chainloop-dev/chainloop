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
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type IntegrationList struct {
	cfg *ActionsOpts
}

type IntegrationItem struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	CreatedAt *time.Time             `json:"createdAt"`
	Config    map[string]interface{} `json:"config"`
}

func NewIntegrationList(cfg *ActionsOpts) *IntegrationList {
	return &IntegrationList{cfg}
}

func (action *IntegrationList) Run() ([]*IntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	resp, err := client.List(context.Background(), &pb.IntegrationsServiceListRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*IntegrationItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbIntegrationItemToAction(p))
	}

	return result, nil
}

func pbIntegrationItemToAction(in *pb.IntegrationItem) *IntegrationItem {
	if in == nil {
		return nil
	}

	i := &IntegrationItem{
		Kind: in.GetKind(), ID: in.GetId(),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
	}

	if c := in.GetConfig().GetDependencyTrack(); c != nil {
		i.Config = map[string]interface{}{
			"host":            c.Domain,
			"allowAutoCreate": c.AllowAutoCreate,
		}
	}

	return i
}
