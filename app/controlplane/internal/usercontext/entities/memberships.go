//
// Copyright 2025 The Chainloop Authors.
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

package entities

import (
	"context"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
)

type Membership struct {
	UserID    uuid.UUID
	Resources []*ResourceMembership
}

type ResourceMembership struct {
	MembershipID uuid.UUID
	Role         authz.Role
	ResourceType authz.ResourceType
	ResourceID   uuid.UUID
}

func WithMembership(ctx context.Context, m *Membership) context.Context {
	return context.WithValue(ctx, membershipCtxKey{}, m)
}

func CurrentMembership(ctx context.Context) *Membership {
	res := ctx.Value(membershipCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*Membership)
}

type membershipCtxKey struct{}
