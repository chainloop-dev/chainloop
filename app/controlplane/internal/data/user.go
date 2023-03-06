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

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/user"
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
	u, err := r.data.db.User.Query().
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
	u, err := r.data.db.User.Create().SetEmail(email).Save(ctx)
	if err != nil {
		return nil, err
	}

	return entUserToBizUser(u), nil
}

// Find by ID, returns nil if not found
func (r *userRepo) FindByID(ctx context.Context, userID uuid.UUID) (*biz.User, error) {
	u, err := r.data.db.User.Get(ctx, userID)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if u == nil {
		return nil, nil
	}

	return entUserToBizUser(u), nil
}

func (r *userRepo) Delete(ctx context.Context, userID uuid.UUID) (err error) {
	return r.data.db.User.DeleteOneID(userID).Exec(ctx)
}

func entUserToBizUser(eu *ent.User) *biz.User {
	return &biz.User{Email: eu.Email, ID: eu.ID.String(), CreatedAt: toTimePtr(eu.CreatedAt)}
}
