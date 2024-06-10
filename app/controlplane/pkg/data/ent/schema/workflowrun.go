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
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// WorkflowRun holds the schema definition for the WorkflowRun entity.
type WorkflowRun struct {
	ent.Schema
}

// Fields of the WorkflowRun.
func (WorkflowRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Time("finished_at").Optional(),
		field.Enum("state").
			GoType(biz.WorkflowRunStatus("")).
			Default(string(biz.WorkflowRunInitialized)),
		field.Text("reason").Optional(),
		field.String("run_url").Optional(),
		field.String("runner_type").Optional(),
		field.JSON("attestation", &dsse.Envelope{}).Optional(),
		field.String("attestation_digest").Optional(),
		field.Bytes("attestation_state").Optional(),
		// The version of the contract that was used
		field.Int("contract_revision_used"),
		// The latest version of the contract that was available
		// at the time of the initialization of the run
		field.Int("contract_revision_latest"),
	}
}

// Edges of the WorkflowRun.
func (WorkflowRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workflow", Workflow.Type).
			Ref("workflowruns").
			Unique(),
		edge.To("contract_version", WorkflowContractVersion.Type).Unique().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		// A WorkflowRun can have multiple CASBackends associated to it
		edge.To("cas_backends", CASBackend.Type),
	}
}

func (WorkflowRun) Indexes() []ent.Index {
	return []ent.Index{
		// Cursor pagination support
		index.Fields("created_at", "id"),
		// Expiration job
		index.Fields("created_at", "state"),
		index.Fields("attestation_digest"),
	}
}
