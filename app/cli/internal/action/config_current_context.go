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

type ConfigCurrentContext struct {
	cfg *ActionsOpts
}

func NewConfigCurrentContext(cfg *ActionsOpts) *ConfigCurrentContext {
	return &ConfigCurrentContext{cfg}
}

type ConfigContextItem struct {
	CurrentUser    *ConfigContextItemUser
	CurrentOrg     *OrgItem
	CurrentOCIRepo *ConfigContextItemOCIRepo
}

type ConfigContextItemUser struct {
	ID, Email string
	CreatedAt *time.Time
}

type ConfigContextItemOCIRepo struct {
	ID, Repo         string
	CreatedAt        *time.Time
	ValidationStatus ValidationStatus
}

type ValidationStatus string

const (
	Valid   ValidationStatus = "valid"
	Invalid ValidationStatus = "invalid"
)

func (action *ConfigCurrentContext) Run() (*ConfigContextItem, error) {
	client := pb.NewContextServiceClient(action.cfg.CPConnecction)
	resp, err := client.Current(context.Background(), &pb.ContextServiceCurrentRequest{})
	if err != nil {
		return nil, err
	}

	res := resp.GetResult()

	item := &ConfigContextItem{
		CurrentUser: &ConfigContextItemUser{
			ID:        res.GetCurrentUser().Id,
			Email:     res.GetCurrentUser().Email,
			CreatedAt: toTimePtr(res.GetCurrentUser().CreatedAt.AsTime()),
		},
		CurrentOrg: pbOrgItemToAction(res.GetCurrentOrg()),
	}

	repo := res.GetCurrentOciRepo()
	if repo != nil {
		r := &ConfigContextItemOCIRepo{
			ID: repo.GetId(), Repo: repo.GetRepo(), CreatedAt: toTimePtr(repo.GetCreatedAt().AsTime()),
		}

		switch repo.GetValidationStatus() {
		case pb.OCIRepositoryItem_VALIDATION_STATUS_OK:
			r.ValidationStatus = Valid
		case pb.OCIRepositoryItem_VALIDATION_STATUS_INVALID:
			r.ValidationStatus = Invalid
		}

		item.CurrentOCIRepo = r
	}
	return item, nil
}
