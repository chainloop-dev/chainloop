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
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type APITokenRepo struct {
	data *Data
	log  *log.Helper
}

func NewAPITokenRepo(data *Data, logger log.Logger) biz.APITokenRepo {
	return &APITokenRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Persist the APIToken to the database.
func (r *APITokenRepo) Create(ctx context.Context, description *string, expiresAt *time.Time, organizationID uuid.UUID) (*biz.APIToken, error) {
	token, err := r.data.db.APIToken.Create().
		SetNillableDescription(description).
		SetNillableExpiresAt(expiresAt).
		SetOrganizationID(organizationID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("saving APIToken: %w", err)
	}

	return entAPITokenToBiz(token), nil
}

func entAPITokenToBiz(t *ent.APIToken) *biz.APIToken {
	return &biz.APIToken{
		ID:             t.ID,
		Description:    t.Description,
		CreatedAt:      toTimePtr(t.CreatedAt),
		ExpiresAt:      toTimePtr(t.ExpiresAt),
		RevokedAt:      toTimePtr(t.RevokedAt),
		OrganizationID: t.OrganizationID,
	}
}
