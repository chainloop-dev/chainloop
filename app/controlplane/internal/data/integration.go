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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/integration"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/integrationattachment"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type IntegrationRepo struct {
	data *Data
	log  *log.Helper
}

func NewIntegrationRepo(data *Data, logger log.Logger) biz.IntegrationRepo {
	return &IntegrationRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *IntegrationRepo) Create(ctx context.Context, orgID uuid.UUID, kind, secretName string, config *v1.IntegrationConfig) (*biz.Integration, error) {
	integration, err := r.data.db.Integration.Create().
		SetOrganizationID(orgID).
		SetKind(kind).
		SetSecretName(secretName).
		SetConfig(config).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	return entIntegrationToBiz(integration), nil
}

func (r *IntegrationRepo) List(ctx context.Context, orgID uuid.UUID) ([]*biz.Integration, error) {
	integrations, err := orgScopedQuery(r.data.db, orgID).
		QueryIntegrations().
		Where(integration.DeletedAtIsNil()).
		Order(ent.Desc(integration.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.Integration, 0, len(integrations))
	for _, i := range integrations {
		result = append(result, entIntegrationToBiz(i))
	}

	return result, nil
}

func (r *IntegrationRepo) FindByIDInOrg(ctx context.Context, orgID, id uuid.UUID) (*biz.Integration, error) {
	integration, err := orgScopedQuery(r.data.db, orgID).
		QueryIntegrations().
		Where(integration.ID(id)).
		Where(integration.DeletedAtIsNil()).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if integration == nil {
		return nil, nil
	}

	return entIntegrationToBiz(integration), nil
}

func (r *IntegrationRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return err
	}

	// soft-delete attachments associated with this workflow
	if err := tx.IntegrationAttachment.Update().Where(integrationattachment.HasIntegrationWith(integration.ID(id))).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
		return err
	}

	if err := tx.Integration.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

func entIntegrationToBiz(i *ent.Integration) *biz.Integration {
	if i == nil {
		return nil
	}

	return &biz.Integration{ID: i.ID, Kind: i.Kind, CreatedAt: toTimePtr(i.CreatedAt), Config: i.Config, SecretName: i.SecretName}
}
