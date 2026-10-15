//
// Copyright 2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/loginhandoff"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
)

var loginHandoffRepoTracer = otelx.Tracer("chainloop-controlplane", "data/loginhandoff")

type LoginHandoffRepo struct {
	data *Data
	log  *log.Helper
}

func NewLoginHandoffRepo(data *Data, logger log.Logger) biz.LoginHandoffRepo {
	return &LoginHandoffRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *LoginHandoffRepo) Create(ctx context.Context, codeHash string, h *biz.LoginHandoff) error {
	ctx, span := otelx.Start(ctx, loginHandoffRepoTracer, "LoginHandoffRepo.Create")
	defer span.End()

	return r.data.DB.LoginHandoff.Create().
		SetCodeHash(codeHash).
		SetUserID(h.UserID).
		SetTokenTTLSeconds(int64(h.TokenTTL / time.Second)).
		SetCodeChallenge(h.CodeChallenge).
		SetCallback(h.Callback).
		SetExpiresAt(h.ExpiresAt).
		Exec(ctx)
}

func (r *LoginHandoffRepo) Consume(ctx context.Context, codeHash string, at time.Time) (*biz.LoginHandoff, bool, error) {
	ctx, span := otelx.Start(ctx, loginHandoffRepoTracer, "LoginHandoffRepo.Consume")
	defer span.End()

	// A single conditional update, so only one concurrent request can mark the code used
	n, err := r.data.DB.LoginHandoff.Update().
		Where(loginhandoff.CodeHash(codeHash), loginhandoff.UsedAtIsNil()).
		SetUsedAt(at).
		Save(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("marking login handoff used: %w", err)
	}

	h, err := r.data.DB.LoginHandoff.Query().Where(loginhandoff.CodeHash(codeHash)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, biz.NewErrNotFound("login handoff")
		}
		return nil, false, fmt.Errorf("finding login handoff: %w", err)
	}

	return entLoginHandoffToBiz(h), n == 1, nil
}

func (r *LoginHandoffRepo) DeleteExpiredOrUsed(ctx context.Context, before time.Time) (int, error) {
	ctx, span := otelx.Start(ctx, loginHandoffRepoTracer, "LoginHandoffRepo.DeleteExpiredOrUsed")
	defer span.End()

	return r.data.DB.LoginHandoff.Delete().
		Where(loginhandoff.Or(loginhandoff.ExpiresAtLT(before), loginhandoff.UsedAtNotNil())).
		Exec(ctx)
}

func entLoginHandoffToBiz(h *ent.LoginHandoff) *biz.LoginHandoff {
	return &biz.LoginHandoff{
		ID:            h.ID,
		UserID:        h.UserID,
		TokenTTL:      time.Duration(h.TokenTTLSeconds) * time.Second,
		CodeChallenge: h.CodeChallenge,
		Callback:      h.Callback,
		ExpiresAt:     h.ExpiresAt,
	}
}
