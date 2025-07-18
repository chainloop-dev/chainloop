// Code generated by ent, DO NOT EDIT.

package ent

import (
	"fmt"
	"strings"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowcontract"
	"github.com/google/uuid"
)

// WorkflowContract is the model entity for the WorkflowContract schema.
type WorkflowContract struct {
	config `json:"-"`
	// ID of the ent.
	ID uuid.UUID `json:"id,omitempty"`
	// Name holds the value of the "name" field.
	Name string `json:"name,omitempty"`
	// CreatedAt holds the value of the "created_at" field.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// DeletedAt holds the value of the "deleted_at" field.
	DeletedAt time.Time `json:"deleted_at,omitempty"`
	// Description holds the value of the "description" field.
	Description string `json:"description,omitempty"`
	// ScopedResourceType holds the value of the "scoped_resource_type" field.
	ScopedResourceType biz.ContractScope `json:"scoped_resource_type,omitempty"`
	// ScopedResourceID holds the value of the "scoped_resource_id" field.
	ScopedResourceID uuid.UUID `json:"scoped_resource_id,omitempty"`
	// Edges holds the relations/edges for other nodes in the graph.
	// The values are being populated by the WorkflowContractQuery when eager-loading is set.
	Edges                           WorkflowContractEdges `json:"edges"`
	organization_workflow_contracts *uuid.UUID
	selectValues                    sql.SelectValues
}

// WorkflowContractEdges holds the relations/edges for other nodes in the graph.
type WorkflowContractEdges struct {
	// Versions holds the value of the versions edge.
	Versions []*WorkflowContractVersion `json:"versions,omitempty"`
	// Organization holds the value of the organization edge.
	Organization *Organization `json:"organization,omitempty"`
	// Workflows holds the value of the workflows edge.
	Workflows []*Workflow `json:"workflows,omitempty"`
	// loadedTypes holds the information for reporting if a
	// type was loaded (or requested) in eager-loading or not.
	loadedTypes [3]bool
}

// VersionsOrErr returns the Versions value or an error if the edge
// was not loaded in eager-loading.
func (e WorkflowContractEdges) VersionsOrErr() ([]*WorkflowContractVersion, error) {
	if e.loadedTypes[0] {
		return e.Versions, nil
	}
	return nil, &NotLoadedError{edge: "versions"}
}

// OrganizationOrErr returns the Organization value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e WorkflowContractEdges) OrganizationOrErr() (*Organization, error) {
	if e.Organization != nil {
		return e.Organization, nil
	} else if e.loadedTypes[1] {
		return nil, &NotFoundError{label: organization.Label}
	}
	return nil, &NotLoadedError{edge: "organization"}
}

// WorkflowsOrErr returns the Workflows value or an error if the edge
// was not loaded in eager-loading.
func (e WorkflowContractEdges) WorkflowsOrErr() ([]*Workflow, error) {
	if e.loadedTypes[2] {
		return e.Workflows, nil
	}
	return nil, &NotLoadedError{edge: "workflows"}
}

// scanValues returns the types for scanning values from sql.Rows.
func (*WorkflowContract) scanValues(columns []string) ([]any, error) {
	values := make([]any, len(columns))
	for i := range columns {
		switch columns[i] {
		case workflowcontract.FieldName, workflowcontract.FieldDescription, workflowcontract.FieldScopedResourceType:
			values[i] = new(sql.NullString)
		case workflowcontract.FieldCreatedAt, workflowcontract.FieldDeletedAt:
			values[i] = new(sql.NullTime)
		case workflowcontract.FieldID, workflowcontract.FieldScopedResourceID:
			values[i] = new(uuid.UUID)
		case workflowcontract.ForeignKeys[0]: // organization_workflow_contracts
			values[i] = &sql.NullScanner{S: new(uuid.UUID)}
		default:
			values[i] = new(sql.UnknownType)
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the WorkflowContract fields.
func (wc *WorkflowContract) assignValues(columns []string, values []any) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case workflowcontract.FieldID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value != nil {
				wc.ID = *value
			}
		case workflowcontract.FieldName:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field name", values[i])
			} else if value.Valid {
				wc.Name = value.String
			}
		case workflowcontract.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field created_at", values[i])
			} else if value.Valid {
				wc.CreatedAt = value.Time
			}
		case workflowcontract.FieldDeletedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field deleted_at", values[i])
			} else if value.Valid {
				wc.DeletedAt = value.Time
			}
		case workflowcontract.FieldDescription:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field description", values[i])
			} else if value.Valid {
				wc.Description = value.String
			}
		case workflowcontract.FieldScopedResourceType:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field scoped_resource_type", values[i])
			} else if value.Valid {
				wc.ScopedResourceType = biz.ContractScope(value.String)
			}
		case workflowcontract.FieldScopedResourceID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field scoped_resource_id", values[i])
			} else if value != nil {
				wc.ScopedResourceID = *value
			}
		case workflowcontract.ForeignKeys[0]:
			if value, ok := values[i].(*sql.NullScanner); !ok {
				return fmt.Errorf("unexpected type %T for field organization_workflow_contracts", values[i])
			} else if value.Valid {
				wc.organization_workflow_contracts = new(uuid.UUID)
				*wc.organization_workflow_contracts = *value.S.(*uuid.UUID)
			}
		default:
			wc.selectValues.Set(columns[i], values[i])
		}
	}
	return nil
}

// Value returns the ent.Value that was dynamically selected and assigned to the WorkflowContract.
// This includes values selected through modifiers, order, etc.
func (wc *WorkflowContract) Value(name string) (ent.Value, error) {
	return wc.selectValues.Get(name)
}

// QueryVersions queries the "versions" edge of the WorkflowContract entity.
func (wc *WorkflowContract) QueryVersions() *WorkflowContractVersionQuery {
	return NewWorkflowContractClient(wc.config).QueryVersions(wc)
}

// QueryOrganization queries the "organization" edge of the WorkflowContract entity.
func (wc *WorkflowContract) QueryOrganization() *OrganizationQuery {
	return NewWorkflowContractClient(wc.config).QueryOrganization(wc)
}

// QueryWorkflows queries the "workflows" edge of the WorkflowContract entity.
func (wc *WorkflowContract) QueryWorkflows() *WorkflowQuery {
	return NewWorkflowContractClient(wc.config).QueryWorkflows(wc)
}

// Update returns a builder for updating this WorkflowContract.
// Note that you need to call WorkflowContract.Unwrap() before calling this method if this WorkflowContract
// was returned from a transaction, and the transaction was committed or rolled back.
func (wc *WorkflowContract) Update() *WorkflowContractUpdateOne {
	return NewWorkflowContractClient(wc.config).UpdateOne(wc)
}

// Unwrap unwraps the WorkflowContract entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (wc *WorkflowContract) Unwrap() *WorkflowContract {
	_tx, ok := wc.config.driver.(*txDriver)
	if !ok {
		panic("ent: WorkflowContract is not a transactional entity")
	}
	wc.config.driver = _tx.drv
	return wc
}

// String implements the fmt.Stringer.
func (wc *WorkflowContract) String() string {
	var builder strings.Builder
	builder.WriteString("WorkflowContract(")
	builder.WriteString(fmt.Sprintf("id=%v, ", wc.ID))
	builder.WriteString("name=")
	builder.WriteString(wc.Name)
	builder.WriteString(", ")
	builder.WriteString("created_at=")
	builder.WriteString(wc.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("deleted_at=")
	builder.WriteString(wc.DeletedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("description=")
	builder.WriteString(wc.Description)
	builder.WriteString(", ")
	builder.WriteString("scoped_resource_type=")
	builder.WriteString(fmt.Sprintf("%v", wc.ScopedResourceType))
	builder.WriteString(", ")
	builder.WriteString("scoped_resource_id=")
	builder.WriteString(fmt.Sprintf("%v", wc.ScopedResourceID))
	builder.WriteByte(')')
	return builder.String()
}

// WorkflowContracts is a parsable slice of WorkflowContract.
type WorkflowContracts []*WorkflowContract
