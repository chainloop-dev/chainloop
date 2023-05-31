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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
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
		i, err := pbIntegrationItemToAction(p)
		if err != nil {
			return nil, err
		}

		result = append(result, i)
	}

	return result, nil
}

func pbIntegrationItemToAction(in *pb.IntegrationItem) (*IntegrationItem, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}

	i := &IntegrationItem{
		Kind: in.GetKind(), ID: in.GetId(),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
	}

	// Old format does not include config so we skip it
	if in.Config == nil {
		return i, nil
	}

	var err error
	if i.Config, err = anyPbToMap(in.Config); err != nil {
		return nil, fmt.Errorf("failed to convert config: %w", err)
	}

	return i, nil
}

func anyPbToMap(in *anypb.Any) (map[string]interface{}, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}

	// proto => JSON
	configJSON, _ := protojson.Marshal(in)

	// JSON => map
	result := make(map[string]interface{})
	if err := json.Unmarshal(configJSON, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// remove the key "@type" belonging to proto.Any
	delete(result, "@type")

	return result, nil
}
