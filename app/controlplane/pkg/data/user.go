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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*biz.User, error) {
	u, err := r.data.DB.User.Query().
		Where(user.Email(email)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if u == nil {
		return nil, nil
	}

	return entUserToBizUser(u), nil
}

func (r *userRepo) CreateByEmail(ctx context.Context, email string) (*biz.User, error) {
	u, err := r.data.DB.User.Create().SetEmail(email).Save(ctx)
	if err != nil {
		return nil, err
	}

	// Query it to load the fully formed object, including proper casted dates that come from the DB
	return r.FindByID(ctx, u.ID)
}

// Find by ID, returns nil if not found
func (r *userRepo) FindByID(ctx context.Context, userID uuid.UUID) (*biz.User, error) {
	u, err := r.data.DB.User.Get(ctx, userID)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if u == nil {
		return nil, nil
	}

	return entUserToBizUser(u), nil
}

func (r *userRepo) Delete(ctx context.Context, userID uuid.UUID) (err error) {
	return r.data.DB.User.DeleteOneID(userID).Exec(ctx)
}

// UpdateAccess updates the access restriction for a user
func (r *userRepo) UpdateAccess(ctx context.Context, userID uuid.UUID, isAccessRestricted bool) error {
	_, err := r.data.DB.User.UpdateOneID(userID).SetHasRestrictedAccess(isAccessRestricted).Save(ctx)
	if err != nil {
		return fmt.Errorf("error updating user access: %w", err)
	}

	return nil
}

// FindAll get all users in the system using pagination
func (r *userRepo) FindAll(ctx context.Context, pagination *pagination.OffsetPaginationOpts) ([]*biz.User, int, error) {
	if pagination == nil {
		return nil, 0, fmt.Errorf("pagination options is required")
	}

	baseQuery := r.data.DB.User.Query()

	count, err := baseQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	users, err := baseQuery.
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Limit(pagination.Limit()).
		Offset(pagination.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*biz.User, 0, len(users))
	for _, u := range users {
		result = append(result, entUserToBizUser(u))
	}

	return result, count, nil
}

// CountUsersWithRestrictedAccess returns the number of users with restricted access
func (r *userRepo) CountUsersWithRestrictedAccess(ctx context.Context) (int, error) {
	return r.data.DB.User.Query().
		Where(user.HasRestrictedAccess(true)).
		Count(ctx)
}

func entUserToBizUser(eu *ent.User) *biz.User {
	return &biz.User{
		Email:               eu.Email,
		ID:                  eu.ID.String(),
		CreatedAt:           toTimePtr(eu.CreatedAt),
		HasRestrictedAccess: eu.HasRestrictedAccess,
	}
}
