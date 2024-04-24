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

package data

import (
	"context"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/robotaccount"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/workflow"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type RobotAccountRepo struct {
	data *Data
	log  *log.Helper
}

func NewRobotAccountRepo(data *Data, logger log.Logger) biz.RobotAccountRepo {
	return &RobotAccountRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *RobotAccountRepo) Create(ctx context.Context, name string, workflowID uuid.UUID) (*biz.RobotAccount, error) {
	p, err := r.data.db.RobotAccount.Create().SetName(name).SetWorkflowID(workflowID).Save(ctx)
	if err != nil {
		return nil, err
	}

	return entRaToBizRa(p, workflowID), nil
}

func (r *RobotAccountRepo) List(ctx context.Context, workflowID uuid.UUID, includeRevoked bool) ([]*biz.RobotAccount, error) {
	raQuery := r.data.db.Workflow.Query().Where(workflow.ID(workflowID)).QueryRobotaccounts()
	if !includeRevoked {
		raQuery = raQuery.Where(robotaccount.RevokedAtIsNil())
	}

	robotAccounts, err := raQuery.All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.RobotAccount, 0, len(robotAccounts))
	for _, a := range robotAccounts {
		result = append(result, entRaToBizRa(a, workflowID))
	}

	return result, nil
}

func (r *RobotAccountRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.RobotAccount, error) {
	p, err := r.data.db.RobotAccount.Get(ctx, id)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if p == nil {
		return nil, nil
	}

	workflowID, err := p.QueryWorkflow().OnlyID(ctx)
	if err != nil {
		return nil, err
	}

	return entRaToBizRa(p, workflowID), nil
}

func (r *RobotAccountRepo) Revoke(ctx context.Context, orgID, id uuid.UUID) error {
	// Find a non-revoked robot account in the scope of the organization
	acc, err := r.data.db.Organization.Query().Where(organization.ID(orgID)).
		QueryWorkflows().
		QueryRobotaccounts().Where(robotaccount.ID(id)).Where(robotaccount.RevokedAtIsNil()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("robot account")
		}
		return err
	}

	// and update it
	return acc.Update().SetRevokedAt(time.Now()).Exec(ctx)
}

func entRaToBizRa(a *ent.RobotAccount, workflowID uuid.UUID) *biz.RobotAccount {
	return &biz.RobotAccount{Name: a.Name, ID: a.ID, CreatedAt: toTimePtr(a.CreatedAt), WorkflowID: workflowID, RevokedAt: toTimePtr(a.RevokedAt)}
}
