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

type CASBackendAdd struct {
	cfg *ActionsOpts
}

type NewCASBackendAddOpts struct {
	Name        string
	Location    string
	Provider    string
	Description string
	Default     bool
	Credentials map[string]any
}

func NewCASBackendAdd(cfg *ActionsOpts) *CASBackendAdd {
	return &CASBackendAdd{cfg}
}

func (action *CASBackendAdd) Run(opts *NewCASBackendAddOpts) (*CASBackendItem, error) {
	// Custom configuration for OCI
	credentials, err := structpb.NewStruct(opts.Credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	client := pb.NewCASBackendServiceClient(action.cfg.CPConnection)
	resp, err := client.Create(context.Background(), &pb.CASBackendServiceCreateRequest{
		Name:        opts.Name,
		Location:    opts.Location,
		Provider:    opts.Provider,
		Description: opts.Description,
		Default:     opts.Default,
		Credentials: credentials,
	})
	if err != nil {
		return nil, err
	}

	return pbCASBackendItemToAction(resp.Result), nil
}
