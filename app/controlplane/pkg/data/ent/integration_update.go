// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/integration"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/integrationattachment"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/google/uuid"
)

// IntegrationUpdate is the builder for updating Integration entities.
type IntegrationUpdate struct {
	config
	hooks     []Hook
	mutation  *IntegrationMutation
	modifiers []func(*sql.UpdateBuilder)
}

// Where appends a list predicates to the IntegrationUpdate builder.
func (iu *IntegrationUpdate) Where(ps ...predicate.Integration) *IntegrationUpdate {
	iu.mutation.Where(ps...)
	return iu
}

// SetName sets the "name" field.
func (iu *IntegrationUpdate) SetName(s string) *IntegrationUpdate {
	iu.mutation.SetName(s)
	return iu
}

// SetNillableName sets the "name" field if the given value is not nil.
func (iu *IntegrationUpdate) SetNillableName(s *string) *IntegrationUpdate {
	if s != nil {
		iu.SetName(*s)
	}
	return iu
}

// SetDescription sets the "description" field.
func (iu *IntegrationUpdate) SetDescription(s string) *IntegrationUpdate {
	iu.mutation.SetDescription(s)
	return iu
}

// SetNillableDescription sets the "description" field if the given value is not nil.
func (iu *IntegrationUpdate) SetNillableDescription(s *string) *IntegrationUpdate {
	if s != nil {
		iu.SetDescription(*s)
	}
	return iu
}

// ClearDescription clears the value of the "description" field.
func (iu *IntegrationUpdate) ClearDescription() *IntegrationUpdate {
	iu.mutation.ClearDescription()
	return iu
}

// SetConfiguration sets the "configuration" field.
func (iu *IntegrationUpdate) SetConfiguration(b []byte) *IntegrationUpdate {
	iu.mutation.SetConfiguration(b)
	return iu
}

// ClearConfiguration clears the value of the "configuration" field.
func (iu *IntegrationUpdate) ClearConfiguration() *IntegrationUpdate {
	iu.mutation.ClearConfiguration()
	return iu
}

// SetDeletedAt sets the "deleted_at" field.
func (iu *IntegrationUpdate) SetDeletedAt(t time.Time) *IntegrationUpdate {
	iu.mutation.SetDeletedAt(t)
	return iu
}

// SetNillableDeletedAt sets the "deleted_at" field if the given value is not nil.
func (iu *IntegrationUpdate) SetNillableDeletedAt(t *time.Time) *IntegrationUpdate {
	if t != nil {
		iu.SetDeletedAt(*t)
	}
	return iu
}

// ClearDeletedAt clears the value of the "deleted_at" field.
func (iu *IntegrationUpdate) ClearDeletedAt() *IntegrationUpdate {
	iu.mutation.ClearDeletedAt()
	return iu
}

// AddAttachmentIDs adds the "attachments" edge to the IntegrationAttachment entity by IDs.
func (iu *IntegrationUpdate) AddAttachmentIDs(ids ...uuid.UUID) *IntegrationUpdate {
	iu.mutation.AddAttachmentIDs(ids...)
	return iu
}

// AddAttachments adds the "attachments" edges to the IntegrationAttachment entity.
func (iu *IntegrationUpdate) AddAttachments(i ...*IntegrationAttachment) *IntegrationUpdate {
	ids := make([]uuid.UUID, len(i))
	for j := range i {
		ids[j] = i[j].ID
	}
	return iu.AddAttachmentIDs(ids...)
}

// SetOrganizationID sets the "organization" edge to the Organization entity by ID.
func (iu *IntegrationUpdate) SetOrganizationID(id uuid.UUID) *IntegrationUpdate {
	iu.mutation.SetOrganizationID(id)
	return iu
}

// SetOrganization sets the "organization" edge to the Organization entity.
func (iu *IntegrationUpdate) SetOrganization(o *Organization) *IntegrationUpdate {
	return iu.SetOrganizationID(o.ID)
}

// Mutation returns the IntegrationMutation object of the builder.
func (iu *IntegrationUpdate) Mutation() *IntegrationMutation {
	return iu.mutation
}

// ClearAttachments clears all "attachments" edges to the IntegrationAttachment entity.
func (iu *IntegrationUpdate) ClearAttachments() *IntegrationUpdate {
	iu.mutation.ClearAttachments()
	return iu
}

// RemoveAttachmentIDs removes the "attachments" edge to IntegrationAttachment entities by IDs.
func (iu *IntegrationUpdate) RemoveAttachmentIDs(ids ...uuid.UUID) *IntegrationUpdate {
	iu.mutation.RemoveAttachmentIDs(ids...)
	return iu
}

// RemoveAttachments removes "attachments" edges to IntegrationAttachment entities.
func (iu *IntegrationUpdate) RemoveAttachments(i ...*IntegrationAttachment) *IntegrationUpdate {
	ids := make([]uuid.UUID, len(i))
	for j := range i {
		ids[j] = i[j].ID
	}
	return iu.RemoveAttachmentIDs(ids...)
}

// ClearOrganization clears the "organization" edge to the Organization entity.
func (iu *IntegrationUpdate) ClearOrganization() *IntegrationUpdate {
	iu.mutation.ClearOrganization()
	return iu
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (iu *IntegrationUpdate) Save(ctx context.Context) (int, error) {
	return withHooks(ctx, iu.sqlSave, iu.mutation, iu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (iu *IntegrationUpdate) SaveX(ctx context.Context) int {
	affected, err := iu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (iu *IntegrationUpdate) Exec(ctx context.Context) error {
	_, err := iu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (iu *IntegrationUpdate) ExecX(ctx context.Context) {
	if err := iu.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (iu *IntegrationUpdate) check() error {
	if _, ok := iu.mutation.OrganizationID(); iu.mutation.OrganizationCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "Integration.organization"`)
	}
	return nil
}

// Modify adds a statement modifier for attaching custom logic to the UPDATE statement.
func (iu *IntegrationUpdate) Modify(modifiers ...func(u *sql.UpdateBuilder)) *IntegrationUpdate {
	iu.modifiers = append(iu.modifiers, modifiers...)
	return iu
}

func (iu *IntegrationUpdate) sqlSave(ctx context.Context) (n int, err error) {
	if err := iu.check(); err != nil {
		return n, err
	}
	_spec := sqlgraph.NewUpdateSpec(integration.Table, integration.Columns, sqlgraph.NewFieldSpec(integration.FieldID, field.TypeUUID))
	if ps := iu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := iu.mutation.Name(); ok {
		_spec.SetField(integration.FieldName, field.TypeString, value)
	}
	if value, ok := iu.mutation.Description(); ok {
		_spec.SetField(integration.FieldDescription, field.TypeString, value)
	}
	if iu.mutation.DescriptionCleared() {
		_spec.ClearField(integration.FieldDescription, field.TypeString)
	}
	if value, ok := iu.mutation.Configuration(); ok {
		_spec.SetField(integration.FieldConfiguration, field.TypeBytes, value)
	}
	if iu.mutation.ConfigurationCleared() {
		_spec.ClearField(integration.FieldConfiguration, field.TypeBytes)
	}
	if value, ok := iu.mutation.DeletedAt(); ok {
		_spec.SetField(integration.FieldDeletedAt, field.TypeTime, value)
	}
	if iu.mutation.DeletedAtCleared() {
		_spec.ClearField(integration.FieldDeletedAt, field.TypeTime)
	}
	if iu.mutation.AttachmentsCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iu.mutation.RemovedAttachmentsIDs(); len(nodes) > 0 && !iu.mutation.AttachmentsCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iu.mutation.AttachmentsIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if iu.mutation.OrganizationCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   integration.OrganizationTable,
			Columns: []string{integration.OrganizationColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(organization.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iu.mutation.OrganizationIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   integration.OrganizationTable,
			Columns: []string{integration.OrganizationColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(organization.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_spec.AddModifiers(iu.modifiers...)
	if n, err = sqlgraph.UpdateNodes(ctx, iu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{integration.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	iu.mutation.done = true
	return n, nil
}

// IntegrationUpdateOne is the builder for updating a single Integration entity.
type IntegrationUpdateOne struct {
	config
	fields    []string
	hooks     []Hook
	mutation  *IntegrationMutation
	modifiers []func(*sql.UpdateBuilder)
}

// SetName sets the "name" field.
func (iuo *IntegrationUpdateOne) SetName(s string) *IntegrationUpdateOne {
	iuo.mutation.SetName(s)
	return iuo
}

// SetNillableName sets the "name" field if the given value is not nil.
func (iuo *IntegrationUpdateOne) SetNillableName(s *string) *IntegrationUpdateOne {
	if s != nil {
		iuo.SetName(*s)
	}
	return iuo
}

// SetDescription sets the "description" field.
func (iuo *IntegrationUpdateOne) SetDescription(s string) *IntegrationUpdateOne {
	iuo.mutation.SetDescription(s)
	return iuo
}

// SetNillableDescription sets the "description" field if the given value is not nil.
func (iuo *IntegrationUpdateOne) SetNillableDescription(s *string) *IntegrationUpdateOne {
	if s != nil {
		iuo.SetDescription(*s)
	}
	return iuo
}

// ClearDescription clears the value of the "description" field.
func (iuo *IntegrationUpdateOne) ClearDescription() *IntegrationUpdateOne {
	iuo.mutation.ClearDescription()
	return iuo
}

// SetConfiguration sets the "configuration" field.
func (iuo *IntegrationUpdateOne) SetConfiguration(b []byte) *IntegrationUpdateOne {
	iuo.mutation.SetConfiguration(b)
	return iuo
}

// ClearConfiguration clears the value of the "configuration" field.
func (iuo *IntegrationUpdateOne) ClearConfiguration() *IntegrationUpdateOne {
	iuo.mutation.ClearConfiguration()
	return iuo
}

// SetDeletedAt sets the "deleted_at" field.
func (iuo *IntegrationUpdateOne) SetDeletedAt(t time.Time) *IntegrationUpdateOne {
	iuo.mutation.SetDeletedAt(t)
	return iuo
}

// SetNillableDeletedAt sets the "deleted_at" field if the given value is not nil.
func (iuo *IntegrationUpdateOne) SetNillableDeletedAt(t *time.Time) *IntegrationUpdateOne {
	if t != nil {
		iuo.SetDeletedAt(*t)
	}
	return iuo
}

// ClearDeletedAt clears the value of the "deleted_at" field.
func (iuo *IntegrationUpdateOne) ClearDeletedAt() *IntegrationUpdateOne {
	iuo.mutation.ClearDeletedAt()
	return iuo
}

// AddAttachmentIDs adds the "attachments" edge to the IntegrationAttachment entity by IDs.
func (iuo *IntegrationUpdateOne) AddAttachmentIDs(ids ...uuid.UUID) *IntegrationUpdateOne {
	iuo.mutation.AddAttachmentIDs(ids...)
	return iuo
}

// AddAttachments adds the "attachments" edges to the IntegrationAttachment entity.
func (iuo *IntegrationUpdateOne) AddAttachments(i ...*IntegrationAttachment) *IntegrationUpdateOne {
	ids := make([]uuid.UUID, len(i))
	for j := range i {
		ids[j] = i[j].ID
	}
	return iuo.AddAttachmentIDs(ids...)
}

// SetOrganizationID sets the "organization" edge to the Organization entity by ID.
func (iuo *IntegrationUpdateOne) SetOrganizationID(id uuid.UUID) *IntegrationUpdateOne {
	iuo.mutation.SetOrganizationID(id)
	return iuo
}

// SetOrganization sets the "organization" edge to the Organization entity.
func (iuo *IntegrationUpdateOne) SetOrganization(o *Organization) *IntegrationUpdateOne {
	return iuo.SetOrganizationID(o.ID)
}

// Mutation returns the IntegrationMutation object of the builder.
func (iuo *IntegrationUpdateOne) Mutation() *IntegrationMutation {
	return iuo.mutation
}

// ClearAttachments clears all "attachments" edges to the IntegrationAttachment entity.
func (iuo *IntegrationUpdateOne) ClearAttachments() *IntegrationUpdateOne {
	iuo.mutation.ClearAttachments()
	return iuo
}

// RemoveAttachmentIDs removes the "attachments" edge to IntegrationAttachment entities by IDs.
func (iuo *IntegrationUpdateOne) RemoveAttachmentIDs(ids ...uuid.UUID) *IntegrationUpdateOne {
	iuo.mutation.RemoveAttachmentIDs(ids...)
	return iuo
}

// RemoveAttachments removes "attachments" edges to IntegrationAttachment entities.
func (iuo *IntegrationUpdateOne) RemoveAttachments(i ...*IntegrationAttachment) *IntegrationUpdateOne {
	ids := make([]uuid.UUID, len(i))
	for j := range i {
		ids[j] = i[j].ID
	}
	return iuo.RemoveAttachmentIDs(ids...)
}

// ClearOrganization clears the "organization" edge to the Organization entity.
func (iuo *IntegrationUpdateOne) ClearOrganization() *IntegrationUpdateOne {
	iuo.mutation.ClearOrganization()
	return iuo
}

// Where appends a list predicates to the IntegrationUpdate builder.
func (iuo *IntegrationUpdateOne) Where(ps ...predicate.Integration) *IntegrationUpdateOne {
	iuo.mutation.Where(ps...)
	return iuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (iuo *IntegrationUpdateOne) Select(field string, fields ...string) *IntegrationUpdateOne {
	iuo.fields = append([]string{field}, fields...)
	return iuo
}

// Save executes the query and returns the updated Integration entity.
func (iuo *IntegrationUpdateOne) Save(ctx context.Context) (*Integration, error) {
	return withHooks(ctx, iuo.sqlSave, iuo.mutation, iuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (iuo *IntegrationUpdateOne) SaveX(ctx context.Context) *Integration {
	node, err := iuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (iuo *IntegrationUpdateOne) Exec(ctx context.Context) error {
	_, err := iuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (iuo *IntegrationUpdateOne) ExecX(ctx context.Context) {
	if err := iuo.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (iuo *IntegrationUpdateOne) check() error {
	if _, ok := iuo.mutation.OrganizationID(); iuo.mutation.OrganizationCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "Integration.organization"`)
	}
	return nil
}

// Modify adds a statement modifier for attaching custom logic to the UPDATE statement.
func (iuo *IntegrationUpdateOne) Modify(modifiers ...func(u *sql.UpdateBuilder)) *IntegrationUpdateOne {
	iuo.modifiers = append(iuo.modifiers, modifiers...)
	return iuo
}

func (iuo *IntegrationUpdateOne) sqlSave(ctx context.Context) (_node *Integration, err error) {
	if err := iuo.check(); err != nil {
		return _node, err
	}
	_spec := sqlgraph.NewUpdateSpec(integration.Table, integration.Columns, sqlgraph.NewFieldSpec(integration.FieldID, field.TypeUUID))
	id, ok := iuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "Integration.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := iuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, integration.FieldID)
		for _, f := range fields {
			if !integration.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != integration.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := iuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := iuo.mutation.Name(); ok {
		_spec.SetField(integration.FieldName, field.TypeString, value)
	}
	if value, ok := iuo.mutation.Description(); ok {
		_spec.SetField(integration.FieldDescription, field.TypeString, value)
	}
	if iuo.mutation.DescriptionCleared() {
		_spec.ClearField(integration.FieldDescription, field.TypeString)
	}
	if value, ok := iuo.mutation.Configuration(); ok {
		_spec.SetField(integration.FieldConfiguration, field.TypeBytes, value)
	}
	if iuo.mutation.ConfigurationCleared() {
		_spec.ClearField(integration.FieldConfiguration, field.TypeBytes)
	}
	if value, ok := iuo.mutation.DeletedAt(); ok {
		_spec.SetField(integration.FieldDeletedAt, field.TypeTime, value)
	}
	if iuo.mutation.DeletedAtCleared() {
		_spec.ClearField(integration.FieldDeletedAt, field.TypeTime)
	}
	if iuo.mutation.AttachmentsCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iuo.mutation.RemovedAttachmentsIDs(); len(nodes) > 0 && !iuo.mutation.AttachmentsCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iuo.mutation.AttachmentsIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   integration.AttachmentsTable,
			Columns: []string{integration.AttachmentsColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(integrationattachment.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if iuo.mutation.OrganizationCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   integration.OrganizationTable,
			Columns: []string{integration.OrganizationColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(organization.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := iuo.mutation.OrganizationIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   integration.OrganizationTable,
			Columns: []string{integration.OrganizationColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(organization.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_spec.AddModifiers(iuo.modifiers...)
	_node = &Integration{config: iuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, iuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{integration.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	iuo.mutation.done = true
	return _node, nil
}