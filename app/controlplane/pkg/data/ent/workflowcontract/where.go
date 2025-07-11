// Code generated by ent, DO NOT EDIT.

package workflowcontract

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/google/uuid"
)

// ID filters vertices based on their ID field.
func ID(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldID, id))
}

// IDEQ applies the EQ predicate on the ID field.
func IDEQ(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldID, id))
}

// IDNEQ applies the NEQ predicate on the ID field.
func IDNEQ(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldID, id))
}

// IDIn applies the In predicate on the ID field.
func IDIn(ids ...uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldID, ids...))
}

// IDNotIn applies the NotIn predicate on the ID field.
func IDNotIn(ids ...uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldID, ids...))
}

// IDGT applies the GT predicate on the ID field.
func IDGT(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldID, id))
}

// IDGTE applies the GTE predicate on the ID field.
func IDGTE(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldID, id))
}

// IDLT applies the LT predicate on the ID field.
func IDLT(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldID, id))
}

// IDLTE applies the LTE predicate on the ID field.
func IDLTE(id uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldID, id))
}

// Name applies equality check predicate on the "name" field. It's identical to NameEQ.
func Name(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldName, v))
}

// CreatedAt applies equality check predicate on the "created_at" field. It's identical to CreatedAtEQ.
func CreatedAt(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldCreatedAt, v))
}

// DeletedAt applies equality check predicate on the "deleted_at" field. It's identical to DeletedAtEQ.
func DeletedAt(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldDeletedAt, v))
}

// Description applies equality check predicate on the "description" field. It's identical to DescriptionEQ.
func Description(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldDescription, v))
}

// ScopedResourceID applies equality check predicate on the "scoped_resource_id" field. It's identical to ScopedResourceIDEQ.
func ScopedResourceID(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldScopedResourceID, v))
}

// NameEQ applies the EQ predicate on the "name" field.
func NameEQ(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldName, v))
}

// NameNEQ applies the NEQ predicate on the "name" field.
func NameNEQ(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldName, v))
}

// NameIn applies the In predicate on the "name" field.
func NameIn(vs ...string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldName, vs...))
}

// NameNotIn applies the NotIn predicate on the "name" field.
func NameNotIn(vs ...string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldName, vs...))
}

// NameGT applies the GT predicate on the "name" field.
func NameGT(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldName, v))
}

// NameGTE applies the GTE predicate on the "name" field.
func NameGTE(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldName, v))
}

// NameLT applies the LT predicate on the "name" field.
func NameLT(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldName, v))
}

// NameLTE applies the LTE predicate on the "name" field.
func NameLTE(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldName, v))
}

// NameContains applies the Contains predicate on the "name" field.
func NameContains(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldContains(FieldName, v))
}

// NameHasPrefix applies the HasPrefix predicate on the "name" field.
func NameHasPrefix(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldHasPrefix(FieldName, v))
}

// NameHasSuffix applies the HasSuffix predicate on the "name" field.
func NameHasSuffix(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldHasSuffix(FieldName, v))
}

// NameEqualFold applies the EqualFold predicate on the "name" field.
func NameEqualFold(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEqualFold(FieldName, v))
}

// NameContainsFold applies the ContainsFold predicate on the "name" field.
func NameContainsFold(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldContainsFold(FieldName, v))
}

// CreatedAtEQ applies the EQ predicate on the "created_at" field.
func CreatedAtEQ(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldCreatedAt, v))
}

// CreatedAtNEQ applies the NEQ predicate on the "created_at" field.
func CreatedAtNEQ(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldCreatedAt, v))
}

// CreatedAtIn applies the In predicate on the "created_at" field.
func CreatedAtIn(vs ...time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldCreatedAt, vs...))
}

// CreatedAtNotIn applies the NotIn predicate on the "created_at" field.
func CreatedAtNotIn(vs ...time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldCreatedAt, vs...))
}

// CreatedAtGT applies the GT predicate on the "created_at" field.
func CreatedAtGT(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldCreatedAt, v))
}

// CreatedAtGTE applies the GTE predicate on the "created_at" field.
func CreatedAtGTE(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldCreatedAt, v))
}

// CreatedAtLT applies the LT predicate on the "created_at" field.
func CreatedAtLT(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldCreatedAt, v))
}

// CreatedAtLTE applies the LTE predicate on the "created_at" field.
func CreatedAtLTE(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldCreatedAt, v))
}

// DeletedAtEQ applies the EQ predicate on the "deleted_at" field.
func DeletedAtEQ(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldDeletedAt, v))
}

// DeletedAtNEQ applies the NEQ predicate on the "deleted_at" field.
func DeletedAtNEQ(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldDeletedAt, v))
}

// DeletedAtIn applies the In predicate on the "deleted_at" field.
func DeletedAtIn(vs ...time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldDeletedAt, vs...))
}

// DeletedAtNotIn applies the NotIn predicate on the "deleted_at" field.
func DeletedAtNotIn(vs ...time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldDeletedAt, vs...))
}

// DeletedAtGT applies the GT predicate on the "deleted_at" field.
func DeletedAtGT(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldDeletedAt, v))
}

// DeletedAtGTE applies the GTE predicate on the "deleted_at" field.
func DeletedAtGTE(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldDeletedAt, v))
}

// DeletedAtLT applies the LT predicate on the "deleted_at" field.
func DeletedAtLT(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldDeletedAt, v))
}

// DeletedAtLTE applies the LTE predicate on the "deleted_at" field.
func DeletedAtLTE(v time.Time) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldDeletedAt, v))
}

// DeletedAtIsNil applies the IsNil predicate on the "deleted_at" field.
func DeletedAtIsNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIsNull(FieldDeletedAt))
}

// DeletedAtNotNil applies the NotNil predicate on the "deleted_at" field.
func DeletedAtNotNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotNull(FieldDeletedAt))
}

// DescriptionEQ applies the EQ predicate on the "description" field.
func DescriptionEQ(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldDescription, v))
}

// DescriptionNEQ applies the NEQ predicate on the "description" field.
func DescriptionNEQ(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldDescription, v))
}

// DescriptionIn applies the In predicate on the "description" field.
func DescriptionIn(vs ...string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldDescription, vs...))
}

// DescriptionNotIn applies the NotIn predicate on the "description" field.
func DescriptionNotIn(vs ...string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldDescription, vs...))
}

// DescriptionGT applies the GT predicate on the "description" field.
func DescriptionGT(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldDescription, v))
}

// DescriptionGTE applies the GTE predicate on the "description" field.
func DescriptionGTE(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldDescription, v))
}

// DescriptionLT applies the LT predicate on the "description" field.
func DescriptionLT(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldDescription, v))
}

// DescriptionLTE applies the LTE predicate on the "description" field.
func DescriptionLTE(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldDescription, v))
}

// DescriptionContains applies the Contains predicate on the "description" field.
func DescriptionContains(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldContains(FieldDescription, v))
}

// DescriptionHasPrefix applies the HasPrefix predicate on the "description" field.
func DescriptionHasPrefix(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldHasPrefix(FieldDescription, v))
}

// DescriptionHasSuffix applies the HasSuffix predicate on the "description" field.
func DescriptionHasSuffix(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldHasSuffix(FieldDescription, v))
}

// DescriptionIsNil applies the IsNil predicate on the "description" field.
func DescriptionIsNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIsNull(FieldDescription))
}

// DescriptionNotNil applies the NotNil predicate on the "description" field.
func DescriptionNotNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotNull(FieldDescription))
}

// DescriptionEqualFold applies the EqualFold predicate on the "description" field.
func DescriptionEqualFold(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEqualFold(FieldDescription, v))
}

// DescriptionContainsFold applies the ContainsFold predicate on the "description" field.
func DescriptionContainsFold(v string) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldContainsFold(FieldDescription, v))
}

// ScopedResourceTypeEQ applies the EQ predicate on the "scoped_resource_type" field.
func ScopedResourceTypeEQ(v biz.ContractScope) predicate.WorkflowContract {
	vc := v
	return predicate.WorkflowContract(sql.FieldEQ(FieldScopedResourceType, vc))
}

// ScopedResourceTypeNEQ applies the NEQ predicate on the "scoped_resource_type" field.
func ScopedResourceTypeNEQ(v biz.ContractScope) predicate.WorkflowContract {
	vc := v
	return predicate.WorkflowContract(sql.FieldNEQ(FieldScopedResourceType, vc))
}

// ScopedResourceTypeIn applies the In predicate on the "scoped_resource_type" field.
func ScopedResourceTypeIn(vs ...biz.ContractScope) predicate.WorkflowContract {
	v := make([]any, len(vs))
	for i := range v {
		v[i] = vs[i]
	}
	return predicate.WorkflowContract(sql.FieldIn(FieldScopedResourceType, v...))
}

// ScopedResourceTypeNotIn applies the NotIn predicate on the "scoped_resource_type" field.
func ScopedResourceTypeNotIn(vs ...biz.ContractScope) predicate.WorkflowContract {
	v := make([]any, len(vs))
	for i := range v {
		v[i] = vs[i]
	}
	return predicate.WorkflowContract(sql.FieldNotIn(FieldScopedResourceType, v...))
}

// ScopedResourceTypeIsNil applies the IsNil predicate on the "scoped_resource_type" field.
func ScopedResourceTypeIsNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIsNull(FieldScopedResourceType))
}

// ScopedResourceTypeNotNil applies the NotNil predicate on the "scoped_resource_type" field.
func ScopedResourceTypeNotNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotNull(FieldScopedResourceType))
}

// ScopedResourceIDEQ applies the EQ predicate on the "scoped_resource_id" field.
func ScopedResourceIDEQ(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldEQ(FieldScopedResourceID, v))
}

// ScopedResourceIDNEQ applies the NEQ predicate on the "scoped_resource_id" field.
func ScopedResourceIDNEQ(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNEQ(FieldScopedResourceID, v))
}

// ScopedResourceIDIn applies the In predicate on the "scoped_resource_id" field.
func ScopedResourceIDIn(vs ...uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIn(FieldScopedResourceID, vs...))
}

// ScopedResourceIDNotIn applies the NotIn predicate on the "scoped_resource_id" field.
func ScopedResourceIDNotIn(vs ...uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotIn(FieldScopedResourceID, vs...))
}

// ScopedResourceIDGT applies the GT predicate on the "scoped_resource_id" field.
func ScopedResourceIDGT(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGT(FieldScopedResourceID, v))
}

// ScopedResourceIDGTE applies the GTE predicate on the "scoped_resource_id" field.
func ScopedResourceIDGTE(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldGTE(FieldScopedResourceID, v))
}

// ScopedResourceIDLT applies the LT predicate on the "scoped_resource_id" field.
func ScopedResourceIDLT(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLT(FieldScopedResourceID, v))
}

// ScopedResourceIDLTE applies the LTE predicate on the "scoped_resource_id" field.
func ScopedResourceIDLTE(v uuid.UUID) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldLTE(FieldScopedResourceID, v))
}

// ScopedResourceIDIsNil applies the IsNil predicate on the "scoped_resource_id" field.
func ScopedResourceIDIsNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldIsNull(FieldScopedResourceID))
}

// ScopedResourceIDNotNil applies the NotNil predicate on the "scoped_resource_id" field.
func ScopedResourceIDNotNil() predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.FieldNotNull(FieldScopedResourceID))
}

// HasVersions applies the HasEdge predicate on the "versions" edge.
func HasVersions() predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := sqlgraph.NewStep(
			sqlgraph.From(Table, FieldID),
			sqlgraph.Edge(sqlgraph.O2M, false, VersionsTable, VersionsColumn),
		)
		sqlgraph.HasNeighbors(s, step)
	})
}

// HasVersionsWith applies the HasEdge predicate on the "versions" edge with a given conditions (other predicates).
func HasVersionsWith(preds ...predicate.WorkflowContractVersion) predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := newVersionsStep()
		sqlgraph.HasNeighborsWith(s, step, func(s *sql.Selector) {
			for _, p := range preds {
				p(s)
			}
		})
	})
}

// HasOrganization applies the HasEdge predicate on the "organization" edge.
func HasOrganization() predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := sqlgraph.NewStep(
			sqlgraph.From(Table, FieldID),
			sqlgraph.Edge(sqlgraph.M2O, true, OrganizationTable, OrganizationColumn),
		)
		sqlgraph.HasNeighbors(s, step)
	})
}

// HasOrganizationWith applies the HasEdge predicate on the "organization" edge with a given conditions (other predicates).
func HasOrganizationWith(preds ...predicate.Organization) predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := newOrganizationStep()
		sqlgraph.HasNeighborsWith(s, step, func(s *sql.Selector) {
			for _, p := range preds {
				p(s)
			}
		})
	})
}

// HasWorkflows applies the HasEdge predicate on the "workflows" edge.
func HasWorkflows() predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := sqlgraph.NewStep(
			sqlgraph.From(Table, FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, WorkflowsTable, WorkflowsColumn),
		)
		sqlgraph.HasNeighbors(s, step)
	})
}

// HasWorkflowsWith applies the HasEdge predicate on the "workflows" edge with a given conditions (other predicates).
func HasWorkflowsWith(preds ...predicate.Workflow) predicate.WorkflowContract {
	return predicate.WorkflowContract(func(s *sql.Selector) {
		step := newWorkflowsStep()
		sqlgraph.HasNeighborsWith(s, step, func(s *sql.Selector) {
			for _, p := range preds {
				p(s)
			}
		})
	})
}

// And groups predicates with the AND operator between them.
func And(predicates ...predicate.WorkflowContract) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.AndPredicates(predicates...))
}

// Or groups predicates with the OR operator between them.
func Or(predicates ...predicate.WorkflowContract) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.OrPredicates(predicates...))
}

// Not applies the not operator on the given predicate.
func Not(p predicate.WorkflowContract) predicate.WorkflowContract {
	return predicate.WorkflowContract(sql.NotPredicates(p))
}
