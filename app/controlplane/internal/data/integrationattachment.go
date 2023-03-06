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

	v1 "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/integrationattachment"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/workflow"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type IntegrationAttachmentRepo struct {
	data *Data
	log  *log.Helper
}

func NewIntegrationAttachmentRepo(data *Data, logger log.Logger) biz.IntegrationAttachmentRepo {
	return &IntegrationAttachmentRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *IntegrationAttachmentRepo) Create(ctx context.Context, integrationID, workflowID uuid.UUID, config *v1.IntegrationAttachmentConfig) (*biz.IntegrationAttachment, error) {
	ia, err := r.data.db.IntegrationAttachment.Create().
		SetWorkflowID(workflowID).
		SetIntegrationID(integrationID).
		SetConfig(config).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	res := entIntegrationAttachmentToBiz(ia)
	if res != nil {
		res.IntegrationID = integrationID
		res.WorkflowID = workflowID
	}

	return res, nil
}

func (r *IntegrationAttachmentRepo) List(ctx context.Context, orgID, workflowID uuid.UUID) ([]*biz.IntegrationAttachment, error) {
	wfQuery := orgScopedQuery(r.data.db, orgID).QueryWorkflows()
	if workflowID != uuid.Nil {
		wfQuery = wfQuery.Where(workflow.ID(workflowID))
	}

	res, err := wfQuery.QueryIntegrationAttachments().WithIntegration().WithWorkflow().
		Where(integrationattachment.DeletedAtIsNil()).
		Order(ent.Desc(integrationattachment.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.IntegrationAttachment, 0, len(res))
	for _, r := range res {
		result = append(result, entIntegrationAttachmentToBiz(r))
	}

	return result, nil
}

func (r *IntegrationAttachmentRepo) FindByIDInOrg(ctx context.Context, orgID, id uuid.UUID) (*biz.IntegrationAttachment, error) {
	integration, err := orgScopedQuery(r.data.db, orgID).
		QueryIntegrations().
		QueryAttachments().
		Where(integrationattachment.DeletedAtIsNil(), integrationattachment.ID(id)).
		WithIntegration().WithWorkflow().
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if integration == nil {
		return nil, nil
	}

	return entIntegrationAttachmentToBiz(integration), nil
}

func (r *IntegrationAttachmentRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.IntegrationAttachment.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

func entIntegrationAttachmentToBiz(i *ent.IntegrationAttachment) *biz.IntegrationAttachment {
	if i == nil {
		return nil
	}

	r := &biz.IntegrationAttachment{ID: i.ID,
		CreatedAt: toTimePtr(i.CreatedAt), Config: i.Config,
	}

	if i.Edges.Workflow != nil {
		r.WorkflowID = i.Edges.Workflow.ID
	}

	if i.Edges.Integration != nil {
		r.IntegrationID = i.Edges.Integration.ID
	}

	return r
}
