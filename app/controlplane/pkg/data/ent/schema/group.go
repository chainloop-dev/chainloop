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

package schema

import (
	"time"

	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/index"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Group struct {
	ent.Schema
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("name").NotEmpty(),
		field.String("description").Optional(),
		field.UUID("organization_id", uuid.UUID{}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("updated_at").
			Default(time.Now).
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("deleted_at").Optional(),
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		// The members of the group
		edge.To("members", User.Type).Through("group_users", GroupMembership.Type),
		// The organization this group belongs to
		edge.From("organization", Organization.Type).
			Field("organization_id").
			Ref("groups").
			Unique().
			Required(),
	}
}

func (Group) Indexes() []ent.Index {
	return []ent.Index{
		// names are unique within an organization and affects only to non-deleted items
		index.Fields("name").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("deleted_at IS NULL"),
		),
	}
}
