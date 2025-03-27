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
	"net/mail"
	"time"

	"entgo.io/ent/schema/index"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").
			Unique().
			Validate(func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			}),
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{
				Default: "CURRENT_TIMESTAMP",
			}),
		field.Bool("has_restricted_access").Default(true).Comment("Whether the user is blocked from accessing the system"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("memberships", Membership.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("has_restricted_access").Annotations(
			entsql.IndexWhere("has_restricted_access IS true"),
		),
	}
}
