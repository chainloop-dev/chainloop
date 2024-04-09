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
		// NOTE: neither the location nor the provider can be updated
		// CAS backend location, i.e S3 bucket name, OCI repository name
		field.String("location").Immutable(),
		field.String("name").Immutable(),
		field.Enum("provider").GoType(biz.CASBackendProvider("")).Immutable(),
		field.String("description").Optional(),
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
		field.Bool("default").Default(false),
		field.Time("deleted_at").Optional(),
		// fallback, main cas backend. If true, this backend will be used as a fallback and cannot be deleted
		field.Bool("fallback").Default(false).Immutable(),
	}
}

func (CASBackend) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("organization", Organization.Type).Ref("cas_backends").Unique().Required(),
		// WorkflowRuns might be associated with multiple CASBackends
		edge.From("workflow_run", WorkflowRun.Type).Ref("cas_backends"),
	}
}

func (CASBackend) Indexes() []ent.Index {
	return []ent.Index{
		// names are unique within a organization and affects only to non-deleted items
		index.Fields("name").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("deleted_at IS NULL"),
		),
	}
}
