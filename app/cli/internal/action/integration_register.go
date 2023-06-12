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

type IntegrationRegister struct {
	cfg *ActionsOpts
}

func NewIntegrationRegister(cfg *ActionsOpts) *IntegrationRegister {
	return &IntegrationRegister{cfg}
}

func (action *IntegrationRegister) Run(kind, description string, options map[string]any) (*IntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	// Transform to structpb for transport
	requestConfig, err := structpb.NewStruct(options)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	i, err := client.Register(context.Background(), &pb.IntegrationsServiceRegisterRequest{
		Kind:        kind,
		Config:      requestConfig,
		DisplayName: description,
	})
	if err != nil {
		return nil, err
	}

	return pbIntegrationItemToAction(i.Result)
}
