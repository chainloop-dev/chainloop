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
)

type RegisteredIntegrationDescribe struct {
	cfg *ActionsOpts
}

func NewRegisteredIntegrationDescribe(cfg *ActionsOpts) *RegisteredIntegrationDescribe {
	return &RegisteredIntegrationDescribe{cfg}
}

func (action *RegisteredIntegrationDescribe) Run(id string) (*RegisteredIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	resp, err := client.DescribeRegistration(context.Background(), &pb.IntegrationsServiceDescribeRegistrationRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}

	i, err := pbRegisteredIntegrationItemToAction(resp.Result)
	if err != nil {
		return nil, err
	}

	return i, nil
}
