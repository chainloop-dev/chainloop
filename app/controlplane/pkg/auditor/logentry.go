package auditor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"
	"time"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
)

// ActorType is the type for the actor of a log entry, meaning the user or service that performed the action.
type ActorType string

// TargetType is the type for the target of a log entry, aka the resource acted upon.
type TargetType string

const (
	// ActorTypeUser is the type for a user actor.
	ActorTypeUser ActorType = "USER"
	// ActorTypeAPIToken is the type for am API Token actor.
	ActorTypeAPIToken ActorType = "API_TOKEN"
	// ActorTypeSystem is the type for a system actor.
	ActorTypeSystem = "SYSTEM"
)

// LogEntry is the interface for all log entries.
// All events should implement this interface to be able to be logged.
type LogEntry interface {
	// ActionType returns the type of the action performed.
	ActionType() string
	// ActionInfo returns the information about the action performed.
	ActionInfo() (json.RawMessage, error)
	// TargetType returns the type of the target of the action.
	TargetType() TargetType
	// TargetID returns the ID of the target of the action.
	TargetID() *uuid.UUID
	// Description returns a templatable string, see the DescriptionVariables struct.
	Description() string
}

type DescriptionVariables struct {
	ActorType  ActorType
	ActorID    *uuid.UUID
	ActorEmail string
	OrgID      *uuid.UUID
}

const AuditEventType = "AUDIT_EVENT"

type EventPayload struct {
	EventType string // AUDIT_EVENT
	Timestamp time.Time
	Data      *AuditEventPayload
}
type AuditEventPayload struct {
	ActionType  string
	TargetType  TargetType
	TargetID    *uuid.UUID
	ActorType   ActorType
	ActorID     *uuid.UUID
	ActorEmail  string
	OrgID       *uuid.UUID
	Description string
	Info        json.RawMessage
	Digest      *cr_v1.Hash
}

func GenerateAuditEvent(entry LogEntry, opts ...GeneratorOption) (*EventPayload, error) {
	options := &GeneratorOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	descriptionTpl := entry.Description()
	if descriptionTpl == "" {
		return nil, errors.New("description is required")
	}

	description, err := interpolateDescription(descriptionTpl, options)
	if err != nil {
		return nil, fmt.Errorf("failed to parse description template: %w", err)
	}

	actionType := entry.ActionType()
	if actionType == "" {
		return nil, errors.New("action type is required")
	}

	targetType := entry.TargetType()
	if targetType == "" {
		return nil, errors.New("target type is required")
	}

	actionInfo, err := entry.ActionInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get action info: %w", err)
	}

	// calculate the digest of the log entry
	digest, err := digest(entry, options.OrgID, options.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate digest: %w", err)
	}

	return &EventPayload{
		EventType: AuditEventType,
		Timestamp: time.Now(),
		Data: &AuditEventPayload{
			Description: description,
			ActionType:  actionType,
			TargetType:  targetType,
			TargetID:    entry.TargetID(),
			Info:        actionInfo,
			ActorType:   options.ActorType,
			ActorID:     options.ActorID,
			ActorEmail:  options.ActorEmail,
			OrgID:       options.OrgID,
			Digest:      digest,
		},
	}, nil
}

func (e *EventPayload) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}

	return data, nil
}

func interpolateDescription(tmplStr string, variables *GeneratorOptions) (string, error) {
	tmpl, err := template.New("description").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse description template: %w", err)
	}

	description := new(bytes.Buffer)
	if err = tmpl.Execute(description, &DescriptionVariables{
		ActorType:  variables.ActorType,
		ActorID:    variables.ActorID,
		ActorEmail: variables.ActorEmail,
		OrgID:      variables.OrgID,
	}); err != nil {
		return "", fmt.Errorf("failed to execute description template: %w", err)
	}

	return description.String(), nil
}

type GeneratorOption func(*GeneratorOptions) error
type GeneratorOptions struct {
	ActorType  ActorType
	ActorID    *uuid.UUID
	ActorEmail string
	OrgID      *uuid.UUID
}

func WithActor(actorType ActorType, actorID uuid.UUID, email string) GeneratorOption {
	return func(a *GeneratorOptions) error {
		if actorType == "" {
			return errors.New("actor type is required")
		}

		if actorType == ActorTypeUser && email == "" {
			return errors.New("email is required")
		}

		a.ActorType = actorType
		// Only set actorID if it is not the zero value
		if actorID != uuid.Nil {
			a.ActorID = &actorID
		}
		// Only set email if it is not empty
		if email != "" {
			a.ActorEmail = email
		}

		return nil
	}
}

func WithOrgID(orgID uuid.UUID) GeneratorOption {
	return func(a *GeneratorOptions) error {
		a.OrgID = &orgID
		return nil
	}
}

// digest generates a hash of the log entry
// that can be used to detect duplicates
func digest(entry LogEntry, orgID *uuid.UUID, userID *uuid.UUID) (*cr_v1.Hash, error) {
	var err error
	toHash := struct {
		ActionType string
		TargetType TargetType
		TargetID   *uuid.UUID
		Info       json.RawMessage
		OrgID      *uuid.UUID
		UserID     *uuid.UUID
	}{
		OrgID:      orgID,
		UserID:     userID,
		ActionType: entry.ActionType(),
		TargetType: entry.TargetType(),
		TargetID:   entry.TargetID(),
	}

	toHash.Info, err = entry.ActionInfo()
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(toHash)
	if err != nil {
		return nil, err
	}

	h, _, err := cr_v1.SHA256(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return &h, nil
}
