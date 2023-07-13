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
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASBackendProvider string

const (
	CASBackendOCI CASBackendProvider = "OCI"
)

type CASBackendValidationStatus string

var CASBackendValidationOK CASBackendValidationStatus = "OK"
var CASBackendValidationFailed CASBackendValidationStatus = "Invalid"

type CASBackend struct {
	ID                     uuid.UUID
	Name, SecretName       string
	CreatedAt, ValidatedAt *time.Time
	OrganizationID         string
	ValidationStatus       CASBackendValidationStatus
	// OCI, S3, ...
	Provider CASBackendProvider
	// Wether this is the default cas backend for the organization
	Default bool
}

type CASBackendOpts struct {
	Name, SecretName string
	Provider         CASBackendProvider
	Default          bool
}

type CASBackendCreateOpts struct {
	*CASBackendOpts
	OrgID uuid.UUID
}

type CASBackendUpdateOpts struct {
	*CASBackendOpts
	ID uuid.UUID
}

type CASBackendRepo interface {
	FindDefaultBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*CASBackend, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*CASBackend, error)
	UpdateValidationStatus(ctx context.Context, ID uuid.UUID, status CASBackendValidationStatus) error
	Create(context.Context, *CASBackendCreateOpts) (*CASBackend, error)
	Update(context.Context, *CASBackendUpdateOpts) (*CASBackend, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type CASBackendReader interface {
	FindDefaultBackend(ctx context.Context, orgID string) (*CASBackend, error)
	FindByID(ctx context.Context, ID string) (*CASBackend, error)
	PerformValidation(ctx context.Context, ID string) error
}

type CASBackendUseCase struct {
	repo               CASBackendRepo
	logger             *log.Helper
	credsRW            credentials.ReaderWriter
	ociBackendProvider backend.Provider
}

func NewCASBackendUseCase(repo CASBackendRepo, credsRW credentials.ReaderWriter, p backend.Provider, l log.Logger) *CASBackendUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &CASBackendUseCase{repo, servicelogger.ScopedHelper(l, "biz/CASBackend"), credsRW, p}
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

	return uc.repo.FindDefaultBackend(ctx, orgUUID)
}

func (uc *CASBackendUseCase) FindByID(ctx context.Context, id string) (*CASBackend, error) {
	backendUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	backend, err := uc.repo.FindByID(ctx, backendUUID)
	if err != nil {
		return nil, err
	} else if backend == nil {
		return nil, NewErrNotFound("CAS Backend")
	}

	return backend, nil
}

func (uc *CASBackendUseCase) Create(ctx context.Context, orgID, name string, provider CASBackendProvider, configJSON []byte, defaultB bool) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// TODO: (miguel) this logic (marshalling from struct + validation) will be moved to the actual backend implementation
	// This endpoint will support other backends in the future
	if provider != CASBackendOCI {
		return nil, NewErrValidation(errors.New("unsupported provider"))
	}

	var ociConfig = struct {
		Password string `json:"password"`
		Username string `json:"username"`
		Repo     string `json:"repo"`
	}{}

	if err := json.Unmarshal(configJSON, &ociConfig); err != nil {
		return nil, NewErrValidation(err)
	}

	// Create and validate credentials
	k, err := ociauth.NewCredentials(ociConfig.Repo, ociConfig.Username, ociConfig.Password)
	if err != nil {
		return nil, NewErrValidation(err)
	}

	// Check credentials
	b, err := oci.NewBackend(ociConfig.Repo, &oci.RegistryOptions{Keychain: k})
	if err != nil {
		return nil, fmt.Errorf("checking credentials: %w", err)
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, NewErrValidation(fmt.Errorf("wrong credentials: %w", err))
	}

	// Validate and store the secret in the external secrets manager
	creds := &credentials.OCIKeypair{Repo: ociConfig.Repo, Username: ociConfig.Username, Password: ociConfig.Password}
	if err := creds.Validate(); err != nil {
		return nil, NewErrValidation(err)
	}

	secretName, err := uc.credsRW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	return uc.repo.Create(ctx, &CASBackendCreateOpts{
		OrgID: orgUUID,
		CASBackendOpts: &CASBackendOpts{
			Name: name, SecretName: secretName, Provider: provider, Default: defaultB,
		},
	})
}

// TODO(miguel): we need to think about the update mechanism and add some guardrails
// for example, we might only allow updating credentials but not the repository itself or the provider
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

	if backend != nil {
		return uc.repo.Update(ctx, &CASBackendUpdateOpts{
			CASBackendOpts: &CASBackendOpts{
				Name: name, SecretName: secretName, Provider: provider, Default: defaultB,
			},
			ID: backend.ID,
		})
	}

	return uc.repo.Create(ctx, &CASBackendCreateOpts{
		OrgID: orgUUID,
		CASBackendOpts: &CASBackendOpts{
			Name: name, SecretName: secretName, Provider: provider,
			Default: defaultB,
		},
	})
}

// Delete will delete the secret in the external secrets manager and the CAS backend from the database
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

	uc.logger.Infow("msg", "deleting CAS backend external secrets", "ID", id, "secretName", backend.SecretName)
	// Delete the secret in the external secrets manager
	if err := uc.credsRW.DeleteCredentials(ctx, backend.SecretName); err != nil {
		return fmt.Errorf("deleting the credentials: %w", err)
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
// TODO: run this process periodically in the background
// TODO: we need to support other kinds of repositories this is for the OCI type
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

	// Currently this code is just for OCI repositories
	if backend.Provider != CASBackendOCI {
		uc.logger.Warnw("msg", "validation not supported for this provider", "ID", id, "provider", backend.Provider)
		return nil
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
	b, err := uc.ociBackendProvider.FromCredentials(ctx, backend.SecretName)
	if err != nil {
		uc.logger.Infow("msg", "credentials not found or invalid", "ID", id)
		return nil
	}

	// 2 - Perform a write validation
	if err = b.CheckWritePermissions(context.TODO()); err != nil {
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
	for _, s := range []CASBackendProvider{CASBackendOCI} {
		kinds = append(kinds, string(s))
	}

	return
}
