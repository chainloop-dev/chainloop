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

type RegisteredIntegrationDelete struct {
	cfg *ActionsOpts
}

func NewRegisteredIntegrationDelete(cfg *ActionsOpts) *RegisteredIntegrationDelete {
	return &RegisteredIntegrationDelete{cfg}
}

func (action *RegisteredIntegrationDelete) Run(id string) error {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	if _, err := client.Deregister(context.Background(), &pb.IntegrationsServiceDeregisterRequest{Id: id}); err != nil {
		return err
	}

	return nil
}
