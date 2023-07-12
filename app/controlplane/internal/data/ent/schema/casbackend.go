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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/google/uuid"
)

// CASBackend holds the schema definition for the CASBackend entity.
type CASBackend struct {
	ent.Schema
}

func (CASBackend) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("name"),
		field.String("secret_name"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Enum("validation_status").
			GoType(biz.CASBackendValidationStatus("")).
			Default(string(biz.CASBackendValidationOK)),
		field.Time("validated_at").Default(time.Now).
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Enum("provider").GoType(biz.CASBackendProvider("")),
		field.Bool("default").Default(false),
	}
}

func (CASBackend) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("organization", Organization.Type).Ref("cas_backends").Unique().Required(),
		// WorkflowRuns might be associated with multiple CASBackends
		edge.From("workflow_run", WorkflowRun.Type).Ref("cas_backends").Required(),
	}
}
