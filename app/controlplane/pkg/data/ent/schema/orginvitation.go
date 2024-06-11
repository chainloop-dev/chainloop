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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/google/uuid"
)

type OrgInvitation struct {
	ent.Schema
}

func (OrgInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("receiver_email").Immutable(),
		field.Enum("status").
			GoType(biz.OrgInvitationStatus("")).
			Default(string(biz.OrgInvitationStatusPending)),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("deleted_at").Optional(),

		// edge fields to be able to access to them directly
		field.UUID("organization_id", uuid.UUID{}),
		field.UUID("sender_id", uuid.UUID{}),
		// Role that will be assigned to the user when they accept the invitation
		field.Enum("role").GoType(authz.Role("")).Optional(),
	}
}

func (OrgInvitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", Organization.Type).Unique().Required().Field("organization_id").Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("sender", User.Type).Unique().Required().Field("sender_id"),
	}
}
