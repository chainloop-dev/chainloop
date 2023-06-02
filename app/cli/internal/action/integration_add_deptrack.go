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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	deptrack "github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1"
	cxpb "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

type IntegrationAddDeptrack struct {
	cfg *ActionsOpts
}

func NewIntegrationAddDeptrack(cfg *ActionsOpts) *IntegrationAddDeptrack {
	return &IntegrationAddDeptrack{cfg}
}

func (action *IntegrationAddDeptrack) Run(host, apiKey string, allowAutoProjectCreation bool) (*IntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	cdxRegistrationRequest := cxpb.RegistrationRequest{
		ApiKey: apiKey,
		Config: &cxpb.RegistrationConfig{
			Domain: host, AllowAutoCreate: allowAutoProjectCreation,
		},
	}

	anyConfig, err := anypb.New(&cdxRegistrationRequest)
	if err != nil {
		return nil, err
	}

	i, err := client.Register(context.Background(), &pb.IntegrationsServiceRegisterRequest{
		Kind:               deptrack.ID,
		RegistrationConfig: anyConfig,
	})
	if err != nil {
		return nil, err
	}

	return pbIntegrationItemToAction(i.Result)
}
