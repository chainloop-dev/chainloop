//
// Copyright 2023-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/google/uuid"
)

var robotAccountRepoTracer = otelx.Tracer("chainloop-controlplane", "data/robotaccount")

type RobotAccountRepo struct {
	data *Data
}

func NewRobotAccountRepo(data *Data) biz.RobotAccountRepo {
	return &RobotAccountRepo{
		data: data,
	}
}

func (r *RobotAccountRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.RobotAccount, error) {
	ctx, span := otelx.Start(ctx, robotAccountRepoTracer, "RobotAccountRepo.FindByID")
	defer span.End()

	p, err := r.data.DB.RobotAccount.Get(ctx, id)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if p == nil {
		return nil, nil
	}

	workflowID, err := p.QueryWorkflow().OnlyID(ctx)
	if err != nil {
		return nil, err
	}

	return &biz.RobotAccount{ID: p.ID, WorkflowID: workflowID, RevokedAt: toTimePtr(p.RevokedAt)}, nil
}
