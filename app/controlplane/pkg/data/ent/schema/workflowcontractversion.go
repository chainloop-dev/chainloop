//
// Copyright 2024 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/google/uuid"
)

// WorkflowContractVersion holds the schema definition for the WorkflowContractVersion entity.
type WorkflowContractVersion struct {
	ent.Schema
}

// Fields of the WorkflowContractVersion.
func (WorkflowContractVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		// binary representation of the contract proto message
		// DEPRECATED in favor of the raw_body field
		field.Bytes("body").Immutable().Optional(),
		// raw_body is the raw representation of the contract in whatever original format it was (json, yaml, ...)
		// it supersedes the body field
		field.Bytes("raw_body").NotEmpty().Immutable(),
		field.Enum("raw_body_format").GoType(unmarshal.RawFormat("")),
		field.Int("revision").Default(1).Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{
				Default: "CURRENT_TIMESTAMP",
			}),
	}
}

// Edges of the WorkflowContractVersion.
func (WorkflowContractVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contract", WorkflowContract.Type).
			Ref("versions").
			Unique(),
	}
}

// Indexes of the WorkflowContractVersion.
func (WorkflowContractVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("contract"),
	}
}
