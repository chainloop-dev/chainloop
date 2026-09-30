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
	"slices"
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
	// ACL policies for this token. Used for authorization checks.
	Policies []*authz.Policy
	// Scope and ScopeID name what the token is scoped to. They are loaded from the row, never
	// from a claim, and are set for every token read from the database.
	Scope   *authz.ResourceType
	ScopeID *uuid.UUID
	// ProjectIDs are the projects a product token reaches, loaded from the row.
	ProjectIDs []uuid.UUID
	// InstanceScope carries the "scope" claim, which holds only authz.ScopeInstanceAdmin. It is
	// unrelated to Scope above.
	InstanceScope string
	// IsSystem marks tokens minted by internal code paths; these are hidden from the public API.
	IsSystem bool
}

// IsResourceScoped reports whether the token is confined to a product, a resource that does not
// live in the control plane database, which reaches the projects in its ProjectIDs. It is false
// for a project token, unlike ResourceScope. It keys on the scope kind, never on scope_id.
func (t *APIToken) IsResourceScoped() bool {
	return t != nil && t.Scope != nil && *t.Scope == authz.ResourceTypeProduct
}

// IsOrgWide reports whether the token acts for the whole organization: confined to neither a
// project nor a product.
func (t *APIToken) IsOrgWide() bool {
	return t != nil && t.ProjectID == nil && !t.IsResourceScoped()
}

// ResourceScope returns the resource the token is confined to, its project or its product, so
// callers can render and authorize it without naming its kind. ok is false for a token acting
// for its whole organization or instance, and for a product scope missing its id. Mirrors
// biz.APIToken.ResourceScope.
func (t *APIToken) ResourceScope() (kind authz.ResourceType, id uuid.UUID, ok bool) {
	switch {
	case t == nil:
		return "", uuid.Nil, false
	case t.ProjectID != nil:
		return authz.ResourceTypeProject, *t.ProjectID, true
	case t.IsResourceScoped() && t.ScopeID != nil:
		return *t.Scope, *t.ScopeID, true
	default:
		return "", uuid.Nil, false
	}
}

// ReachableProjects returns the projects a confined token reaches: its project, or its list. It
// is never nil for a confined token, and nil for an organization-wide one, which no list confines.
func (t *APIToken) ReachableProjects() []uuid.UUID {
	switch {
	case t == nil || t.IsOrgWide():
		return nil
	case t.ProjectID != nil:
		return []uuid.UUID{*t.ProjectID}
	default:
		return append(make([]uuid.UUID, 0, len(t.ProjectIDs)), t.ProjectIDs...)
	}
}

// ReachesProject reports whether a confined token reaches the project, without copying its
// project list.
func (t *APIToken) ReachesProject(id uuid.UUID) bool {
	switch {
	case t == nil || t.IsOrgWide():
		return false
	case t.ProjectID != nil:
		return *t.ProjectID == id
	default:
		return slices.Contains(t.ProjectIDs, id)
	}
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
