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
		// Optional description
		field.String("description").Optional(),
		field.Time("created_at").Default(time.Now).Immutable().Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("expires_at").Optional().Immutable(),
		// the token can be manually revoked
		field.Time("revoked_at").Optional(),
		field.UUID("organization_id", uuid.UUID{}),
	}
}

func (APIToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", Organization.Type).Unique().Required().Field("organization_id"),
	}
}
