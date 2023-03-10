// Code generated by ent, DO NOT EDIT.

package integration

import (
	"time"

	"github.com/google/uuid"
)

const (
	// Label holds the string label denoting the integration type in the database.
	Label = "integration"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldKind holds the string denoting the kind field in the database.
	FieldKind = "kind"
	// FieldSecretName holds the string denoting the secret_name field in the database.
	FieldSecretName = "secret_name"
	// FieldCreatedAt holds the string denoting the created_at field in the database.
	FieldCreatedAt = "created_at"
	// FieldConfig holds the string denoting the config field in the database.
	FieldConfig = "config"
	// FieldDeletedAt holds the string denoting the deleted_at field in the database.
	FieldDeletedAt = "deleted_at"
	// EdgeAttachments holds the string denoting the attachments edge name in mutations.
	EdgeAttachments = "attachments"
	// EdgeOrganization holds the string denoting the organization edge name in mutations.
	EdgeOrganization = "organization"
	// Table holds the table name of the integration in the database.
	Table = "integrations"
	// AttachmentsTable is the table that holds the attachments relation/edge.
	AttachmentsTable = "integration_attachments"
	// AttachmentsInverseTable is the table name for the IntegrationAttachment entity.
	// It exists in this package in order to avoid circular dependency with the "integrationattachment" package.
	AttachmentsInverseTable = "integration_attachments"
	// AttachmentsColumn is the table column denoting the attachments relation/edge.
	AttachmentsColumn = "integration_attachment_integration"
	// OrganizationTable is the table that holds the organization relation/edge.
	OrganizationTable = "integrations"
	// OrganizationInverseTable is the table name for the Organization entity.
	// It exists in this package in order to avoid circular dependency with the "organization" package.
	OrganizationInverseTable = "organizations"
	// OrganizationColumn is the table column denoting the organization relation/edge.
	OrganizationColumn = "organization_integrations"
)

// Columns holds all SQL columns for integration fields.
var Columns = []string{
	FieldID,
	FieldKind,
	FieldSecretName,
	FieldCreatedAt,
	FieldConfig,
	FieldDeletedAt,
}

// ForeignKeys holds the SQL foreign-keys that are owned by the "integrations"
// table and are not defined as standalone fields in the schema.
var ForeignKeys = []string{
	"organization_integrations",
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
			return true
		}
	}
	for i := range ForeignKeys {
		if column == ForeignKeys[i] {
			return true
		}
	}
	return false
}

var (
	// DefaultCreatedAt holds the default value on creation for the "created_at" field.
	DefaultCreatedAt func() time.Time
	// DefaultID holds the default value on creation for the "id" field.
	DefaultID func() uuid.UUID
)
