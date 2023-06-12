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
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type IntegrationAddDeptrack struct {
	cfg *ActionsOpts
}

func NewIntegrationAddDeptrack(cfg *ActionsOpts) *IntegrationAddDeptrack {
	return &IntegrationAddDeptrack{cfg}
}

func (action *IntegrationAddDeptrack) Run(host, apiKey, description string, allowAutoProjectCreation bool) (*IntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	config := make(map[string]any)
	config["instanceURI"] = host
	config["apiKey"] = apiKey
	config["allowAutoCreate"] = allowAutoProjectCreation

	// Transform to structpb for transport
	requestConfig, err := structpb.NewStruct(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	i, err := client.Register(context.Background(), &pb.IntegrationsServiceRegisterRequest{
		Kind:        "dependencytrack",
		Config:      requestConfig,
		DisplayName: description,
	})
	if err != nil {
		return nil, err
	}

	return pbIntegrationItemToAction(i.Result)
}
