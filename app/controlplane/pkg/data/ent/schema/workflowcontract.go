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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/google/uuid"
)

// WorkflowContract holds the schema definition for the WorkflowContract entity.
type WorkflowContract struct {
	ent.Schema
}

// Fields of the WorkflowContract.
func (WorkflowContract) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("name").Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{
				Default: "CURRENT_TIMESTAMP",
			}),
		field.Time("deleted_at").Optional(),
		field.String("description").Optional(),
		// If this value is set, the contract is scoped to a resource
		field.Enum("scoped_resource_type").GoType(biz.ContractScope("")).Optional(),
		field.UUID("scoped_resource_id", uuid.UUID{}).Optional(),
	}
}

// Edges of the WorkflowContract.
func (WorkflowContract) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", WorkflowContractVersion.Type),
		// We keep the organization edge to be able to easily list all the contracts
		// regardless of the scope
		edge.From("organization", Organization.Type).
			Ref("workflow_contracts").
			Unique(),
		// A contract can be associated to multiple workflows
		edge.From("workflows", Workflow.Type).Ref("contract"),
	}
}

func (WorkflowContract) Indexes() []ent.Index {
	return []ent.Index{
		// TODO: add a unique index on name and scoped_resource_type and scoped_resource_id
		// for now keeping a global one for backward compatibility
		index.Fields("name").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("deleted_at IS NULL"),
		),
	}
}
