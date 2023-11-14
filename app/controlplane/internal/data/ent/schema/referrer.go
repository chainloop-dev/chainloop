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

package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Referrer struct {
	ent.Schema
}

func (Referrer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("digest").Immutable(),
		// referrer kind i.e CONTAINER
		field.String("kind").Immutable(),
		// wether it can be downloaded from CAS or not
		field.Bool("downloadable").Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
	}
}

func (Referrer) Edges() []ent.Edge {
	return []ent.Edge{
		// M2M referrer can refer to itself via references
		edge.To("references", Referrer.Type).From("referred_by").Immutable(),
		// M2M. referrer can be part of multiple organizations
		edge.To("organizations", Organization.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		// M2M. referrer can be part of multiple workflows
		edge.To("workflows", Workflow.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (Referrer) Indexes() []ent.Index {
	return []ent.Index{
		// Two referrers of different kinds can have the same digest
		// For now we only guarantee that the digest is unique in the scope of the referrer kind
		index.Fields("digest", "kind").Unique(),
	}
}
