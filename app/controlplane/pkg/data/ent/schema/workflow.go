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
	"github.com/google/uuid"
)

// Workflow holds the schema definition for the Workflow entity.
type Workflow struct {
	ent.Schema
}

// Fields of the Workflow.
func (Workflow) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Immutable(),
		field.String("project").Optional(),
		field.String("team").Optional(),
		field.Int("runs_count").Default(0),
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{
				Default: "CURRENT_TIMESTAMP",
			}),
		field.Time("deleted_at").Optional(),
		// public means that the workflow runs, attestations and materials are reachable
		field.Bool("public").Default(false),
		field.UUID("organization_id", uuid.UUID{}),
		field.String("description").Optional(),
	}
}

// Edges of the Workflow.
func (Workflow) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("robotaccounts", RobotAccount.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("workflowruns", WorkflowRun.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.From("organization", Organization.Type).Field("organization_id").Ref("workflows").Unique().Required(),
		edge.To("contract", WorkflowContract.Type).Unique().Required(),
		edge.From("integration_attachments", IntegrationAttachment.Type).
			Ref("workflow"),

		// M2M. referrer can be part of multiple workflows
		edge.From("referrers", Referrer.Type).Ref("workflows"),
	}
}

func (Workflow) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "project").Edges("organization").Unique().Annotations(
			entsql.IndexWhere("deleted_at IS NULL"),
		),
		index.Fields("organization_id", "id").Unique().Annotations(
			entsql.IndexWhere("deleted_at IS NULL"),
		),
	}
}
