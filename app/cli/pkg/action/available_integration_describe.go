//
// Copyright 2024 The Chainloop Authors.
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

type AvailableIntegrationDescribe struct {
	cfg *ActionsOpts
}

func NewAvailableIntegrationDescribe(cfg *ActionsOpts) *AvailableIntegrationDescribe {
	return &AvailableIntegrationDescribe{cfg}
}

func (action *AvailableIntegrationDescribe) Run(name string) (*AvailableIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	resp, err := client.ListAvailable(context.Background(), &pb.IntegrationsServiceListAvailableRequest{})
	if err != nil {
		return nil, err
	}

	for _, i := range resp.Result {
		if i.Name == name {
			return pbAvailableIntegrationItemToAction(i)
		}
	}

	return nil, nil
}
