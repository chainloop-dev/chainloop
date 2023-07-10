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

// OCIRepository holds the schema definition for the OCIRepository entity.
type OCIRepository struct {
	ent.Schema
}

// Fields of the OCIRepository.
func (OCIRepository) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Unique(),
		field.String("repo"),
		field.String("secret_name"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.Enum("validation_status").
			GoType(biz.OCIRepoValidationStatus("")).
			Default(string(biz.OCIRepoValidationOK)),
		field.Time("validated_at").Default(time.Now).
			Annotations(&entsql.Annotation{Default: "CURRENT_TIMESTAMP"}),
		field.String("provider").Optional(),
	}
}

// Edges of the OCIRepository.
func (OCIRepository) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("organization", Organization.Type).Ref("oci_repositories").Unique().Required(),
	}
}
