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

package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
)

type APIToken struct {
	ent.Schema
}

// Fields of the APIToken.
func (APIToken) Fields() []ent.Field {
	return []ent.Field{
		// API token identifier
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("name").Immutable(),
		// Optional description
		field.String("description").Optional(),
		field.Time("created_at").Default(time.Now).Immutable().Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("expires_at").Optional(),
		// the token can be manually revoked
		field.Time("revoked_at").Optional(),
		field.Time("last_used_at").Optional(),
		// if this value is not set, the token is an instance-level token
		field.UUID("organization_id", uuid.UUID{}).Optional(),
		// Tokens can be associated with a project
		// if this value is not set, the token is an organization level token
		field.UUID("project_id", uuid.UUID{}).Optional(),
		// Tokens can additionally be scoped to a specific workflow within a project.
		// Only meaningful when project_id is also set.
		field.UUID("workflow_id", uuid.UUID{}).Optional(),
		// Scope kind for tokens confined to a resource that does not live in this database.
		// Only "product" is written today, by the Chainloop platform. The resource itself is
		// not an entity here, so scope_id is a bare UUID with no foreign key — the same
		// arrangement cas_mappings.product_id uses, and for the same reason.
		field.Enum("scope").GoType(authz.ResourceType("")).Optional().Nillable(),
		field.UUID("scope_id", uuid.UUID{}).Optional().Nillable(),
		// ACL policies for this token. NULL means role-based token (future), non-NULL means ACL mode.
		// When set, contains the list of policies this token is allowed to perform.
		field.JSON("policies", []*authz.Policy{}).Optional(),
		// System tokens are minted by internal code paths and hidden from the public API.
		field.Bool("is_system").Default(false).Immutable(),
	}
}

// Annotations keeps the scope columns coherent in the database itself. The rows that carry
// them are written by the Chainloop platform, i.e. from outside this module, so the
// application-level checks in biz.APITokenUseCase.Create cannot be the only thing standing
// between a half-written scope and the authorization path.
//
// Both halves matter because the gate functions key on scope_id: a row with scope set but
// scope_id NULL reads as unconfined, and a row carrying a project_id as well would have its
// project confinement skipped.
func (APIToken) Annotations() []schema.Annotation {
	return []schema.Annotation{
		//nolint:gosec // G101 false positive: these are CHECK expressions, not credentials
		entsql.Checks(map[string]string{
			"apitoken_scope_all_or_nothing":   "(scope IS NULL) = (scope_id IS NULL)",
			"apitoken_scope_excludes_project": "project_id IS NULL OR scope_id IS NULL",
		}),
	}
}

func (APIToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("organization", Organization.Type).Field("organization_id").Ref("api_tokens").Unique(),
		edge.To("project", Project.Type).Field("project_id").Unique(),
		edge.To("workflow", Workflow.Type).Field("workflow_id").Unique(),
	}
}

func (APIToken) Indexes() []ent.Index {
	return []ent.Index{
		// names are unique within a organization and affects only to non-deleted items
		// These are for org level tokens, which are confined to neither a project nor a
		// scoped resource
		index.Fields("name").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND project_id IS NULL AND scope_id IS NULL"),
		),

		// for scope-confined tokens, names are unique within their scoped resource
		index.Fields("name", "scope_id").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND scope_id IS NOT NULL"),
		),

		// for project level tokens, we scope the uniqueness to the organization and project
		index.Fields("name").Edges("project").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND project_id IS NOT NULL"),
		),

		// for instance-level tokens, names must be unique across all instance tokens
		index.Fields("name").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND organization_id IS NULL"),
		),
	}
}
