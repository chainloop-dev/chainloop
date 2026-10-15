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
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// LoginHandoff is a short-lived, single-use code that a login delivers to the client callback
// instead of the session token. It holds no token, only the hash of the code.
type LoginHandoff struct {
	ent.Schema
}

func (LoginHandoff) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		// SHA-256 of the handoff code, hex encoded
		field.String("code_hash").Immutable().Unique(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		// Lifetime of the session token created at exchange time
		field.Int64("token_ttl_seconds").Immutable(),
		// PKCE S256 challenge sent when the login started
		field.String("code_challenge").Immutable(),
		field.String("callback").Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("used_at").Optional().Nillable(),
	}
}

func (LoginHandoff) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Immutable().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (LoginHandoff) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("expires_at"),
	}
}
