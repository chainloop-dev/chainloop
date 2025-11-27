//
// Copyright 2024-2025 The Chainloop Authors.
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

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASBackendProvider string

const (
	// Inline, embedded CAS backend
	CASBackendInline                CASBackendProvider = "INLINE"
	CASBackendInlineDefaultMaxBytes int64              = 500 * 1024       // 500KB
	MinCASBackendMaxBytes           int64              = 10 * 1024 * 1024 // 10MB minimum
	errMsgCredentialsAccess                            = "Failed to access CAS backend credentials in external Secrets Manager"
	errMsgCredentialsFormat                            = "Invalid CAS backend credentials format from external Secrets Manager"
)

var CASBackendInlineDescription = "Embed artifacts content in the attestation (fallback)"

type CASBackendValidationStatus string

var CASBackendValidationOK CASBackendValidationStatus = "OK"
var CASBackendValidationFailed CASBackendValidationStatus = "Invalid"

type CASBackend struct {
	ID                                uuid.UUID
	Name                              string
	Location, Description, SecretName string
	CreatedAt, UpdatedAt, ValidatedAt *time.Time
	OrganizationID                    uuid.UUID
	ValidationStatus                  CASBackendValidationStatus
	ValidationError                   *string
	// OCI, S3, ...
	Provider CASBackendProvider
	// Whether this is the default cas backend for the organization
	Default bool
	// it's a inline backend, the artifacts are embedded in the attestation
	Inline bool
	// It's a fallback backend, it cannot be deleted
	Fallback bool

	Limits *CASBackendLimits
}

type CASBackendLimits struct {
	// Max number of bytes allowed to be stored in this backend per blob
	MaxBytes int64
}

type CASBackendOpts struct {
	OrgID                uuid.UUID
	Location, SecretName string
	Description          *string
	Provider             CASBackendProvider
	Default              *bool
	ValidationStatus     CASBackendValidationStatus
	ValidationError      *string
}

type CASBackendCreateOpts struct {
	*CASBackendOpts
	Name     string
	Fallback bool
	MaxBytes int64
}

type CASBackendUpdateOpts struct {
	*CASBackendOpts
	ID       uuid.UUID
	MaxBytes *int64
}

type CASBackendRepo interface {
	FindDefaultBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindFallbackBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*CASBackend, error)
	FindByIDInOrg(ctx context.Context, OrgID, ID uuid.UUID) (*CASBackend, error)
	FindByNameInOrg(ctx context.Context, OrgID uuid.UUID, name string) (*CASBackend, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*CASBackend, error)
	UpdateValidationStatus(ctx context.Context, ID uuid.UUID, status CASBackendValidationStatus, validationError *string) error
	// ListBackends returns CAS backends across all organizations
	// If onlyDefaults is true, only default backends are returned
	ListBackends(ctx context.Context, onlyDefaults bool) ([]*CASBackend, error)
	Create(context.Context, *CASBackendCreateOpts) (*CASBackend, error)
	Update(context.Context, *CASBackendUpdateOpts) (*CASBackend, error)
	Delete(ctx context.Context, ID uuid.UUID) error
	SoftDelete(ctx context.Context, ID uuid.UUID) error
}

type CASBackendReader interface {
	FindDefaultBackend(ctx context.Context, orgID string) (*CASBackend, error)
	FindByIDInOrg(ctx context.Context, OrgID, ID string) (*CASBackend, error)
	PerformValidation(ctx context.Context, ID string) error
}

type CASBackendUseCase struct {
	repo            CASBackendRepo
	logger          *log.Helper
	credsRW         credentials.ReaderWriter
	providers       backend.Providers
	MaxBytesDefault int64
	auditorUC       *AuditorUseCase
}

// CASServerDefaultOpts holds the default options for the CAS server
type CASServerDefaultOpts struct {
	DefaultEntryMaxSize string
}

func NewCASBackendUseCase(repo CASBackendRepo, credsRW credentials.ReaderWriter, providers backend.Providers, c *CASServerDefaultOpts, auditorUC *AuditorUseCase, l log.Logger) (*CASBackendUseCase, error) {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	var maxBytesDefault uint64 = 100 * 1024 * 1024 // 100MB
	if c != nil && c.DefaultEntryMaxSize != "" {
		var err error
		maxBytesDefault, err = bytefmt.ToBytes(c.DefaultEntryMaxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid CAS backend default max bytes: %w", err)
		}
	}

	return &CASBackendUseCase{
		repo:            repo,
		logger:          servicelogger.ScopedHelper(l, "biz/CASBackend"),
		credsRW:         credsRW,
		providers:       providers,
		MaxBytesDefault: int64(maxBytesDefault),
		auditorUC:       auditorUC,
	}, nil
}

func (uc *CASBackendUseCase) List(ctx context.Context, orgID string) ([]*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	return uc.repo.List(ctx, orgUUID)
}

func (uc *CASBackendUseCase) FindDefaultBackend(ctx context.Context, orgID string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindDefaultBackend(ctx, orgUUID)
	if err != nil {
		return nil, err
	} else if backend == nil {
		return nil, NewErrNotFound("CAS Backend")
	}

	return backend, nil
}

func (uc *CASBackendUseCase) FindByIDInOrg(ctx context.Context, orgID, id string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backendUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindByIDInOrg(ctx, orgUUID, backendUUID)
	if err != nil {
		return nil, err
	} else if backend == nil {
		return nil, NewErrNotFound("CAS Backend")
	}

	return backend, nil
}

func (uc *CASBackendUseCase) FindByNameInOrg(ctx context.Context, orgID, name string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindByNameInOrg(ctx, orgUUID, name)
}

func (uc *CASBackendUseCase) FindFallbackBackend(ctx context.Context, orgID string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindFallbackBackend(ctx, orgUUID)
	if err != nil {
		return nil, err
	} else if backend == nil {
		return nil, NewErrNotFound("CAS Backend")
	}

	return backend, nil
}

func (uc *CASBackendUseCase) CreateInlineFallbackBackend(ctx context.Context, orgID string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.Create(ctx, &CASBackendCreateOpts{
		Name:     "default-inline",
		Fallback: true,
		MaxBytes: CASBackendInlineDefaultMaxBytes,
		CASBackendOpts: &CASBackendOpts{
			Provider: CASBackendInline, Default: ToPtr(true),
			Description: &CASBackendInlineDescription,
			OrgID:       orgUUID,
		},
	})
}

// Set fallback backend as default
func (uc *CASBackendUseCase) defaultFallbackBackend(ctx context.Context, orgID string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindFallbackBackend(ctx, orgUUID)
	if err != nil {
		return nil, err
	} else if backend == nil {
		// If there is no fallback backend, we skip the update
		return nil, nil
	}

	return uc.repo.Update(ctx, &CASBackendUpdateOpts{ID: backend.ID, CASBackendOpts: &CASBackendOpts{Default: ToPtr(true)}})
}

func (uc *CASBackendUseCase) Create(ctx context.Context, orgID, name, location, description string, provider CASBackendProvider, creds any, defaultB bool, maxBytes *int64) (*CASBackend, error) {
	if orgID == "" || name == "" {
		return nil, NewErrValidationStr("organization and name are required")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// validate format of the name and the project
	if err := ValidateIsDNS1123(name); err != nil {
		return nil, NewErrValidation(err)
	}

	// Determine max bytes: use provided value or default
	finalMaxBytes := uc.MaxBytesDefault
	if maxBytes != nil {
		if *maxBytes < MinCASBackendMaxBytes {
			return nil, NewErrValidationStr(fmt.Sprintf("max_bytes must be at least %s", bytefmt.ByteSize(uint64(MinCASBackendMaxBytes))))
		}
		finalMaxBytes = *maxBytes
	}

	secretName, err := uc.credsRW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	backend, err := uc.repo.Create(ctx, &CASBackendCreateOpts{
		MaxBytes: finalMaxBytes,
		Name:     name,
		CASBackendOpts: &CASBackendOpts{
			Location: location, SecretName: secretName, Provider: provider, Default: ToPtr(defaultB),
			Description: &description,
			OrgID:       orgUUID,
		},
	})

	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrAlreadyExistsStr("name already taken")
		}
		return nil, fmt.Errorf("failed to create CAS backend: %w", err)
	}

	// Record CAS backend creation in audit log
	if uc.auditorUC != nil {
		uc.auditorUC.Dispatch(ctx, &events.CASBackendCreated{
			CASBackendBase: &events.CASBackendBase{
				CASBackendID:   &backend.ID,
				CASBackendName: backend.Name,
				Provider:       string(backend.Provider),
				Location:       backend.Location,
				Default:        backend.Default,
			},
			CASBackendDescription: description,
		}, &orgUUID)
	}

	return backend, nil
}

// Update will update credentials, description, default status, or max bytes
func (uc *CASBackendUseCase) Update(ctx context.Context, orgID, id string, description *string, creds any, defaultB *bool, maxBytes *int64) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	before, err := uc.repo.FindByIDInOrg(ctx, orgUUID, uuid)
	if err != nil {
		return nil, err
	} else if before == nil {
		return nil, NewErrNotFound("CAS Backend")
	}

	// Inline backends cannot have their max_bytes updated
	if maxBytes != nil && before.Inline {
		return nil, NewErrValidationStr("inline backends cannot have their max_bytes updated")
	}

	// Validate max_bytes if provided
	if maxBytes != nil && *maxBytes < MinCASBackendMaxBytes {
		return nil, NewErrValidationStr(fmt.Sprintf("max_bytes must be at least %s", bytefmt.ByteSize(uint64(MinCASBackendMaxBytes))))
	}

	var secretName string
	credentialsUpdated := creds != nil
	// We want to rotate credentials
	if creds != nil {
		secretName, err = uc.credsRW.SaveCredentials(ctx, orgID, creds)
		if err != nil {
			return nil, fmt.Errorf("storing the credentials: %w", err)
		}
	}

	// Update the backend without modifying validation status directly
	// The validation status will be updated through PerformValidation if needed
	// Don't set validation status here - let PerformValidation handle it
	updateOpts := &CASBackendUpdateOpts{
		ID:       uuid,
		MaxBytes: maxBytes,
		CASBackendOpts: &CASBackendOpts{
			SecretName:  secretName,
			Default:     defaultB,
			Description: description,
			OrgID:       orgUUID,
		},
	}

	// If we're not updating credentials, preserve the current validation status
	if !credentialsUpdated {
		updateOpts.ValidationStatus = before.ValidationStatus
		updateOpts.ValidationError = before.ValidationError
	}

	after, err := uc.repo.Update(ctx, updateOpts)
	if err != nil {
		return nil, err
	}

	// If credentials were updated, perform validation to check if they work
	// This will properly update validation status and send events
	if credentialsUpdated {
		if err := uc.PerformValidation(ctx, id); err != nil {
			// Log the validation error but don't fail the update operation
			// The validation status will be updated by PerformValidation
			uc.logger.Warnw("msg", "validation failed after credential update", "ID", id, "error", err)
		}

		// Reload the backend to get the updated validation status
		after, err = uc.repo.FindByIDInOrg(ctx, orgUUID, uuid)
		if err != nil {
			return nil, fmt.Errorf("reloading backend after validation: %w", err)
		}
	}

	// If we just updated the backend from default=true => default=false, we need to set up the fallback as default
	if before.Default && !after.Default {
		if _, err := uc.defaultFallbackBackend(ctx, orgID); err != nil {
			return nil, fmt.Errorf("setting the fallback backend as default: %w", err)
		}
	}

	// Record CAS backend update in audit log
	if uc.auditorUC != nil {
		uc.auditorUC.Dispatch(ctx, &events.CASBackendUpdated{
			CASBackendBase: &events.CASBackendBase{
				CASBackendID:   &after.ID,
				CASBackendName: after.Name,
				Provider:       string(after.Provider),
				Location:       after.Location,
				Default:        after.Default,
			},
			NewDescription:     description,
			CredentialsChanged: credentialsUpdated,
			PreviousDefault:    before.Default,
		}, &orgUUID)
	}

	return after, nil
}

// Deprecated: use Create and update methods separately instead
func (uc *CASBackendUseCase) CreateOrUpdate(ctx context.Context, orgID, name, username, password string, provider CASBackendProvider, defaultB bool) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Validate and store the secret in the external secrets manager
	creds := &credentials.OCIKeypair{Repo: name, Username: username, Password: password}
	if err := creds.Validate(); err != nil {
		return nil, NewErrValidation(err)
	}

	secretName, err := uc.credsRW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	// Check if it already exists, if it does we update it
	// We do not support more than one repository per organization yet
	backend, err := uc.repo.FindDefaultBackend(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("checking for existing CAS backends: %w", err)
	}

	if backend != nil && backend.Provider == provider {
		return uc.repo.Update(ctx, &CASBackendUpdateOpts{
			CASBackendOpts: &CASBackendOpts{
				Location: name, SecretName: secretName, Provider: provider, Default: ToPtr(defaultB),
			},
			ID: backend.ID,
		})
	}

	return uc.repo.Create(ctx, &CASBackendCreateOpts{
		CASBackendOpts: &CASBackendOpts{
			Location: name, SecretName: secretName, Provider: provider,
			Default: ToPtr(defaultB),
			OrgID:   orgUUID,
		},
	})
}

// SoftDelete will mark the cas backend as deleted but will not delete the secret in the external secrets manager
// We keep it so it can be restored or referenced in the future while trying to download an asset
func (uc *CASBackendUseCase) SoftDelete(ctx context.Context, orgID, id string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	backendUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Make sure the backend exists in the organization
	backend, err := uc.repo.FindByIDInOrg(ctx, orgUUID, backendUUID)
	if err != nil {
		return err
	} else if backend == nil {
		return NewErrNotFound("CAS Backend")
	}

	if backend.Fallback {
		return NewErrValidation(errors.New("can't delete the fallback CAS backend"))
	}

	if err := uc.repo.SoftDelete(ctx, backendUUID); err != nil {
		return err
	}

	// If we just deleted the default backend, we need to set up the fallback as default
	if backend.Default {
		if _, err := uc.defaultFallbackBackend(ctx, orgID); err != nil {
			return fmt.Errorf("setting the fallback backend as default: %w", err)
		}
	}

	// Record CAS backend soft deletion in audit log
	if uc.auditorUC != nil {
		uc.auditorUC.Dispatch(ctx, &events.CASBackendDeleted{
			CASBackendBase: &events.CASBackendBase{
				CASBackendID:   &backend.ID,
				CASBackendName: backend.Name,
				Provider:       string(backend.Provider),
				Location:       backend.Location,
				Default:        backend.Default,
			},
		}, &orgUUID)
	}

	return nil
}

// Delete will delete the secret in the external secrets manager and the CAS backend from the database
// This method is used during user off-boarding
func (uc *CASBackendUseCase) Delete(ctx context.Context, id string) error {
	uc.logger.Infow("msg", "deleting CAS Backend", "ID", id)

	backendUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindByID(ctx, backendUUID)
	if err != nil {
		return err
	} else if backend == nil {
		return NewErrNotFound("CAS Backend")
	}

	if !backend.Inline {
		uc.logger.Infow("msg", "deleting CAS backend external secrets", "ID", id, "secretName", backend.SecretName)
		// Delete the secret in the external secrets manager
		if err := uc.credsRW.DeleteCredentials(ctx, backend.SecretName); err != nil {
			uc.logger.Errorw("msg", "deleting CAS backend external secrets", "ID", id, "secretName", backend.SecretName, "error", err)
		}
	}

	if delErr := uc.repo.Delete(ctx, backendUUID); delErr != nil {
		return delErr
	}
	uc.logger.Infow("msg", "CAS Backend deleted", "ID", id)

	// Record CAS backend permanent deletion in audit log
	if uc.auditorUC != nil {
		uc.auditorUC.Dispatch(ctx, &events.CASBackendPermanentDeleted{
			CASBackendBase: &events.CASBackendBase{
				CASBackendID:   &backend.ID,
				CASBackendName: backend.Name,
				Provider:       string(backend.Provider),
				Location:       backend.Location,
				Default:        backend.Default,
			},
		}, &backend.OrganizationID)
	}

	return nil
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (CASBackendValidationStatus) Values() (kinds []string) {
	for _, s := range []CASBackendValidationStatus{CASBackendValidationOK, CASBackendValidationFailed} {
		kinds = append(kinds, string(s))
	}

	return
}

// Validate that the repository is valid and reachable
func (uc *CASBackendUseCase) PerformValidation(ctx context.Context, id string) error {
	validationStatus := CASBackendValidationFailed
	var validationError *string

	backendUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindByID(ctx, backendUUID)
	if err != nil {
		return err
	} else if backend == nil {
		return NewErrNotFound("CAS Backend")
	}

	if backend.Provider == CASBackendInline {
		// Inline CAS backend does not need validation
		return nil
	}

	provider, ok := uc.providers[string(backend.Provider)]
	if !ok {
		return fmt.Errorf("CAS backend provider not found: %s", backend.Provider)
	}

	defer func() {
		// reload the backend in case some other instance updated it while we were validating
		backend, err := uc.repo.FindByID(ctx, backendUUID)
		if err != nil {
			uc.logger.Errorw("msg", "finding backend after validation", "ID", id, "error", err)
			return
		} else if backend == nil {
			uc.logger.Errorw("msg", "backend not found after validation", "ID", id)
			return
		}

		// Store previous status for audit logging
		previousStatus := backend.ValidationStatus

		// Update the validation status and error
		uc.logger.Infow("msg", "updating validation status", "ID", id, "status", validationStatus, "error", validationError)
		if err := uc.repo.UpdateValidationStatus(ctx, backendUUID, validationStatus, validationError); err != nil {
			uc.logger.Errorw("msg", "updating validation status", "ID", id, "error", err)
			return
		}

		// Log status change as an audit event if status has changed and auditor is available
		if uc.auditorUC != nil && previousStatus != validationStatus {
			uc.logger.Infow("msg", "status changed, dispatching audit event",
				"backend", backend.ID,
				"previousStatus", previousStatus,
				"newStatus", validationStatus)

			// Check if this is a recovery event (going from failed to OK)
			isRecovery := previousStatus == CASBackendValidationFailed && validationStatus == CASBackendValidationOK

			var validationErrorStr string
			if validationError != nil {
				validationErrorStr = *validationError
			}

			// Create and send event for the status change
			uc.auditorUC.Dispatch(ctx, &events.CASBackendStatusChanged{
				CASBackendBase: &events.CASBackendBase{
					CASBackendID:   &backend.ID,
					CASBackendName: backend.Name,
					Provider:       string(backend.Provider),
					Location:       backend.Location,
					Default:        backend.Default,
				},
				PreviousStatus: string(previousStatus),
				NewStatus:      string(validationStatus),
				StatusError:    validationErrorStr,
				IsRecovery:     isRecovery,
			}, &backend.OrganizationID)
		}
	}()

	// 1 - Retrieve the credentials from the external secrets manager
	var creds any
	if err := uc.credsRW.ReadCredentials(ctx, backend.SecretName, &creds); err != nil {
		uc.logger.Infow("msg", "credentials not found or invalid", "ID", id, "error", err)
		validationError = ToPtr(errMsgCredentialsAccess)
		return nil
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		uc.logger.Infow("msg", "credentials invalid", "ID", id, "error", err)
		validationError = ToPtr(errMsgCredentialsFormat)
		return nil
	}

	// 2 - run validation
	_, err = provider.ValidateAndExtractCredentials(backend.Location, credsJSON)
	if err != nil {
		errMsg := err.Error()
		validationError = &errMsg
		uc.logger.Infow("msg", "permissions validation failed", "ID", id, "error", err)
		return nil
	}

	// If everything went well, update the validation status to OK
	validationStatus = CASBackendValidationOK
	validationError = nil
	uc.logger.Infow("msg", "validation OK", "ID", id)

	return nil
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (CASBackendProvider) Values() (kinds []string) {
	for _, s := range []CASBackendProvider{azureblob.ProviderID, oci.ProviderID, CASBackendInline, s3.ProviderID} {
		kinds = append(kinds, string(s))
	}

	return
}
