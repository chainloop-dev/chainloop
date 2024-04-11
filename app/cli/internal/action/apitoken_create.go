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
	"errors"
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

type APITokenCreate struct {
	cfg *ActionsOpts
}

func NewAPITokenCreate(cfg *ActionsOpts) *APITokenCreate {
	return &APITokenCreate{cfg}
}

func (action *APITokenCreate) Run(ctx context.Context, name, description string, expiresIn *time.Duration) (*APITokenItem, error) {
	client := pb.NewAPITokenServiceClient(action.cfg.CPConnection)

	req := &pb.APITokenServiceCreateRequest{Name: name, Description: &description}
	if expiresIn != nil {
		req.ExpiresIn = durationpb.New(*expiresIn)
	}

	resp, err := client.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creating API token: %w", err)
	}

	p := resp.Result
	if p == nil {
		return nil, errors.New("not found")
	}

	item := pbAPITokenItemToAPITokenItem(p.Item)
	item.JWT = p.Jwt

	return item, nil
}

type APITokenItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// JWT is returned only during the creation
	JWT       string     `json:"jwt,omitempty"`
	CreatedAt *time.Time `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func pbAPITokenItemToAPITokenItem(p *pb.APITokenItem) *APITokenItem {
	if p == nil {
		return nil
	}

	item := &APITokenItem{
		ID:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   toTimePtr(p.CreatedAt.AsTime()),
	}

	if p.RevokedAt != nil {
		item.RevokedAt = toTimePtr(p.RevokedAt.AsTime())
	}

	if p.ExpiresAt != nil {
		item.ExpiresAt = toTimePtr(p.ExpiresAt.AsTime())
	}

	return item
}
