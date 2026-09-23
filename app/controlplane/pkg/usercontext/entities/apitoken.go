//
// Copyright 2024-2026 The Chainloop Authors.
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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
)

type APIToken struct {
	ID string
	// Token Name
	Name         string
	CreatedAt    *time.Time
	Token        string
	ProjectID    *uuid.UUID
	ProjectName  *string
	WorkflowID   *uuid.UUID
	WorkflowName *string
	// Scope confines the token to a resource that does not live in the control plane
	// database, such as a product. It is loaded from the token row, never from a claim, and
	// the projects such a token reaches are its rows in the memberships table.
	Scope   *authz.ResourceType
	ScopeID *uuid.UUID
	// ACL policies for this token. Used for authorization checks.
	Policies []*authz.Policy
	// InstanceScope carries the "scope" claim, which today holds only
	// authz.ScopeInstanceAdmin. It is unrelated to Scope above.
	InstanceScope string
	// IsSystem marks tokens minted by internal code paths; these are hidden from the public API.
	IsSystem bool
}

// IsResourceScoped reports whether the token is confined to a resource that does not live in
// the control plane database, such as a product. Such a token authorizes from its memberships
// the way a person does, rather than from a column on the token row.
//
// NOTE: this keys on the scope column, never on whether memberships exist. Deleting a product
// removes its memberships, and a token whose product was deleted must stay confined rather
// than widen to the whole organization.
func (t *APIToken) IsResourceScoped() bool {
	return t != nil && t.ScopeID != nil
}

// IsOrgWide reports whether the token acts for the whole organization, i.e. is confined to
// neither a project nor a resource outside this database.
func (t *APIToken) IsOrgWide() bool {
	return t != nil && t.ProjectID == nil && t.ScopeID == nil
}

func WithCurrentAPIToken(ctx context.Context, token *APIToken) context.Context {
	return context.WithValue(ctx, currentAPITokenCtxKey{}, token)
}

func CurrentAPIToken(ctx context.Context) *APIToken {
	res := ctx.Value(currentAPITokenCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*APIToken)
}

type currentAPITokenCtxKey struct{}
