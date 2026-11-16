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

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ReferrerReference is the edge schema of the references between referrers. A row says that
// referrer_id references referred_by_id.
type ReferrerReference struct {
	ent.Schema
}

func (ReferrerReference) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("referrer_id", uuid.UUID{}).Immutable(),
		field.UUID("referred_by_id", uuid.UUID{}).Immutable(),
		// The creation time of the referenced referrer (referred_by_id), copied so that the
		// references of a referrer can be read newest first from an index. There is no Go default,
		// so a create must set it. The database default covers writers outside Ent's create, such
		// as the previous release during an upgrade, with the time the reference was stored.
		field.Time("created_at").
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
	}
}

func (ReferrerReference) Annotations() []schema.Annotation {
	return []schema.Annotation{
		field.ID("referrer_id", "referred_by_id"),
		entsql.Annotation{Table: "referrer_references"},
	}
}

func (ReferrerReference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("referrer", Referrer.Type).
			Required().
			Unique().
			Immutable().
			Field("referrer_id").
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("referred_by", Referrer.Type).
			Required().
			Unique().
			Immutable().
			Field("referred_by_id").
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (ReferrerReference) Indexes() []ent.Index {
	return []ent.Index{
		// A page of the references of a referrer, newest first, in index order.
		index.Fields("referrer_id", "created_at", "referred_by_id").
			Annotations(entsql.DescColumns("created_at", "referred_by_id")),
	}
}
