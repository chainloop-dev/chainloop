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
		// What the token is scoped to, and the id of that resource. Every new token records it;
		// only a product scope drives any logic for now, and rows from before these columns
		// existed leave both NULL. A product is not an entity here, so scope_id is a bare UUID
		// with no foreign key — the same arrangement cas_mappings.product_id uses.
		field.Enum("scope").GoType(authz.ResourceType("")).Optional().Nillable(),
		field.UUID("scope_id", uuid.UUID{}).Optional().Nillable(),
		// ACL policies for this token. NULL means role-based token (future), non-NULL means ACL mode.
		// When set, contains the list of policies this token is allowed to perform.
		field.JSON("policies", []*authz.Policy{}).Optional(),
		// System tokens are minted by internal code paths and hidden from the public API.
		field.Bool("is_system").Default(false).Immutable(),
	}
}

// Annotations keeps the scope columns coherent in the database itself. The rows that carry a
// product scope are written by the Chainloop platform, i.e. from outside this module, so the
// application-level checks in biz.APITokenUseCase.Create cannot be the only thing standing
// between a malformed scope and the authorization path.
//
// A scope must agree with the row it is on: an organization or project scope names the
// token's own organization or project, an instance scope has no id, and a product scope is
// never combined with a project, whose confinement would otherwise be skipped. Rows from
// before these columns existed carry no scope and pass untouched.
func (APIToken) Annotations() []schema.Annotation {
	return []schema.Annotation{
		//nolint:gosec // G101 false positive: these are CHECK expressions, not credentials
		entsql.Checks(map[string]string{
			"apitoken_scope_id_presence": "(scope_id IS NOT NULL) = (scope IS NOT NULL AND scope <> 'instance')",
			"apitoken_scope_matches_token": "scope IS NULL" +
				" OR (scope = 'organization' AND project_id IS NULL AND scope_id IS NOT DISTINCT FROM organization_id)" +
				" OR (scope = 'project' AND scope_id IS NOT DISTINCT FROM project_id)" +
				" OR (scope = 'instance' AND organization_id IS NULL AND project_id IS NULL)" +
				" OR (scope = 'product' AND organization_id IS NOT NULL AND project_id IS NULL)",
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
		// product. Keyed on the kind, not on scope_id, so rows written before and after the
		// scope columns existed share one namespace
		index.Fields("name").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND project_id IS NULL AND (scope IS NULL OR scope <> 'product')"),
		),

		// for product-scoped tokens, names are unique within their product
		index.Fields("name", "scope_id").Unique().Annotations(
			entsql.IndexWhere("revoked_at IS NULL AND scope = 'product'"),
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
