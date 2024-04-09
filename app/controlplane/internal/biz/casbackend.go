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

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/azureblob"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/s3"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASBackendProvider string

const (
	CASBackendDefaultMaxBytes int64 = 100 * 1024 * 1024 // 100MB
	// Inline, embedded CAS backend
	CASBackendInline                CASBackendProvider = "INLINE"
	CASBackendInlineDefaultMaxBytes int64              = 500 * 1024 // 500KB
)

type CASBackendValidationStatus string

var CASBackendValidationOK CASBackendValidationStatus = "OK"
var CASBackendValidationFailed CASBackendValidationStatus = "Invalid"

type CASBackend struct {
	ID                                uuid.UUID
	Name                              string
	Location, Description, SecretName string
	CreatedAt, ValidatedAt            *time.Time
	OrganizationID                    uuid.UUID
	ValidationStatus                  CASBackendValidationStatus
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
	// Max number of bytes allowed to be stored in this backend
	MaxBytes int64
}

type CASBackendOpts struct {
	OrgID                                   uuid.UUID
	Location, SecretName, Description, Name string
	Provider                                CASBackendProvider
	Default                                 bool
}

type CASBackendCreateOpts struct {
	*CASBackendOpts
	Fallback bool
}

type CASBackendUpdateOpts struct {
	*CASBackendOpts
	ID uuid.UUID
}

type CASBackendRepo interface {
	FindDefaultBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindFallbackBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*CASBackend, error)
	FindByIDInOrg(ctx context.Context, OrgID, ID uuid.UUID) (*CASBackend, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*CASBackend, error)
	UpdateValidationStatus(ctx context.Context, ID uuid.UUID, status CASBackendValidationStatus) error
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
	repo      CASBackendRepo
	logger    *log.Helper
	credsRW   credentials.ReaderWriter
	providers backend.Providers
}

func NewCASBackendUseCase(repo CASBackendRepo, credsRW credentials.ReaderWriter, providers backend.Providers, l log.Logger) *CASBackendUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &CASBackendUseCase{repo, servicelogger.ScopedHelper(l, "biz/CASBackend"), credsRW, providers}
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
		Fallback: true,
		CASBackendOpts: &CASBackendOpts{
			Name:     "default-inline",
			Provider: CASBackendInline, Default: true,
			Description: "Embed artifacts content in the attestation (fallback)",
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

	return uc.repo.Update(ctx, &CASBackendUpdateOpts{ID: backend.ID, CASBackendOpts: &CASBackendOpts{Default: true}})
}

func (uc *CASBackendUseCase) Create(ctx context.Context, orgID, name, location, description string, provider CASBackendProvider, creds any, defaultB bool) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// validate format of the name and the project
	if err := ValidateIsDNS1123(name); err != nil {
		return nil, NewErrValidation(err)
	}

	secretName, err := uc.credsRW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	backend, err := uc.repo.Create(ctx, &CASBackendCreateOpts{
		CASBackendOpts: &CASBackendOpts{
			Location: location, SecretName: secretName, Provider: provider, Default: defaultB,
			Description: description,
			OrgID:       orgUUID,
			Name:        name,
		},
	})

	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("name already taken")
		}
		return nil, fmt.Errorf("failed to create CAS backend: %w", err)
	}

	return backend, nil
}

// Update will update credentials, description or default status
func (uc *CASBackendUseCase) Update(ctx context.Context, orgID, id, description string, creds any, defaultB bool) (*CASBackend, error) {
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

	var secretName string
	// We want to rotate credentials
	if creds != nil {
		secretName, err = uc.credsRW.SaveCredentials(ctx, orgID, creds)
		if err != nil {
			return nil, fmt.Errorf("storing the credentials: %w", err)
		}
	}

	after, err := uc.repo.Update(ctx, &CASBackendUpdateOpts{
		ID: uuid,
		CASBackendOpts: &CASBackendOpts{
			SecretName: secretName, Default: defaultB, Description: description, OrgID: orgUUID,
		},
	})
	if err != nil {
		return nil, err
	}

	// If we just updated the backend from default=true => default=false, we need to set up the fallback as default
	if before.Default && !after.Default {
		if _, err := uc.defaultFallbackBackend(ctx, orgID); err != nil {
			return nil, fmt.Errorf("setting the fallback backend as default: %w", err)
		}
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
				Location: name, SecretName: secretName, Provider: provider, Default: defaultB,
			},
			ID: backend.ID,
		})
	}

	return uc.repo.Create(ctx, &CASBackendCreateOpts{
		CASBackendOpts: &CASBackendOpts{
			Location: name, SecretName: secretName, Provider: provider,
			Default: defaultB,
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

	uc.logger.Infow("msg", "CAS Backend deleted", "ID", id)
	return uc.repo.Delete(ctx, backendUUID)
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (CASBackendValidationStatus) Values() (kinds []string) {
	for _, s := range []CASBackendValidationStatus{CASBackendValidationOK, CASBackendValidationFailed} {
		kinds = append(kinds, string(s))
	}

	return
}

// Validate that the repository is valid and reachable
func (uc *CASBackendUseCase) PerformValidation(ctx context.Context, id string) (err error) {
	validationStatus := CASBackendValidationFailed

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
		// If the actual validation logic failed we do not update the underlying repository
		if err != nil {
			return
		}

		// Update the validation status
		uc.logger.Infow("msg", "updating validation status", "ID", id, "status", validationStatus)
		if err := uc.repo.UpdateValidationStatus(ctx, backendUUID, validationStatus); err != nil {
			uc.logger.Errorw("msg", "updating validation status", "ID", id, "error", err)
		}
	}()

	// 1 - Retrieve the credentials from the external secrets manager
	var creds any
	if err := uc.credsRW.ReadCredentials(ctx, backend.SecretName, &creds); err != nil {
		uc.logger.Infow("msg", "credentials not found or invalid", "ID", id)
		return nil
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		uc.logger.Infow("msg", "credentials invalid", "ID", id)
		return nil
	}

	// 2 - run validation
	_, err = provider.ValidateAndExtractCredentials(backend.Location, credsJSON)
	if err != nil {
		uc.logger.Infow("msg", "permissions validation failed", "ID", id)
		return nil
	}

	// If everything went well, update the validation status to OK
	validationStatus = CASBackendValidationOK
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
