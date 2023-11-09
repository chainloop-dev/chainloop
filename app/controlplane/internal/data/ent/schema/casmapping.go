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

// CASBackend holds the schema definition for the CASBackend entity.
type CASMapping struct {
	ent.Schema
}

func (CASMapping) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("digest").Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
	}
}

func (CASMapping) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("cas_backend", CASBackend.Type).Unique().Required().Immutable().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("workflow_run", WorkflowRun.Type).Unique().Immutable().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("organization", Organization.Type).Unique().Required().Immutable().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (CASMapping) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("digest"),
	}
}
