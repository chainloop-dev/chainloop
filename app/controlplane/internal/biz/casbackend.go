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
	"errors"
	"fmt"
	"io"
	"time"

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASBackendProvider string

const (
	CASBackendOCI CASBackendProvider = "OCI"
)

type CASBackendValidationStatus string

var OCIRepoValidationOK CASBackendValidationStatus = "OK"
var OCIRepoValidationFailed CASBackendValidationStatus = "Invalid"

type CASBackend struct {
	ID, Name, SecretName   string
	CreatedAt, ValidatedAt *time.Time
	OrganizationID         string
	ValidationStatus       CASBackendValidationStatus
	// OCI, S3, ...
	Provider CASBackendProvider
}

type OCIRepoOpts struct {
	Repository, Username, Password, SecretName string
	Provider                                   CASBackendProvider
}

type OCIRepoCreateOpts struct {
	*OCIRepoOpts
	OrgID uuid.UUID
}

type OCIRepoUpdateOpts struct {
	*OCIRepoOpts
	ID uuid.UUID
}

type CASBackendRepo interface {
	FindMainBackend(ctx context.Context, orgID uuid.UUID) (*CASBackend, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*CASBackend, error)
	UpdateValidationStatus(ctx context.Context, ID uuid.UUID, status CASBackendValidationStatus) error
	Create(context.Context, *OCIRepoCreateOpts) (*CASBackend, error)
	Update(context.Context, *OCIRepoUpdateOpts) (*CASBackend, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type CASBackendReader interface {
	FindMainBackend(ctx context.Context, orgID string) (*CASBackend, error)
	FindByID(ctx context.Context, ID string) (*CASBackend, error)
	PerformValidation(ctx context.Context, ID string) error
}

type CASBackendUseCase struct {
	repo               CASBackendRepo
	logger             *log.Helper
	credsRW            credentials.ReaderWriter
	ociBackendProvider backend.Provider
}

func NewOCIRepositoryUseCase(repo CASBackendRepo, credsRW credentials.ReaderWriter, p backend.Provider, l log.Logger) *CASBackendUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &CASBackendUseCase{repo, servicelogger.ScopedHelper(l, "biz/ocirepository"), credsRW, p}
}

var ErrAlreadyRepoInOrg = errors.New("there is already an OCI repository associated with this organization")

func (uc *CASBackendUseCase) FindMainBackend(ctx context.Context, orgID string) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindMainBackend(ctx, orgUUID)
}

func (uc *CASBackendUseCase) FindByID(ctx context.Context, id string) (*CASBackend, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	repo, err := uc.repo.FindByID(ctx, repoUUID)
	if err != nil {
		return nil, err
	} else if repo == nil {
		return nil, NewErrNotFound("OCI repository")
	}

	return repo, nil
}

// TODO(miguel): we need to think about the update mechanism and add some guardrails
// for example, we might only allow updating credentials but not the repository itself or the provider
func (uc *CASBackendUseCase) CreateOrUpdate(ctx context.Context, orgID, repoURL, username, password string, provider CASBackendProvider) (*CASBackend, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Validate and store the secret in the external secrets manager
	creds := &credentials.OCIKeypair{Repo: repoURL, Username: username, Password: password}
	if err := creds.Validate(); err != nil {
		return nil, NewErrValidation(err)
	}

	secretName, err := uc.credsRW.SaveCredentials(ctx, orgID, creds)
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	// Check if it already exists, if it does we update it
	// We do not support more than one repository per organization yet
	repo, err := uc.repo.FindMainBackend(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("checking for existing repositories: %w", err)
	}

	if repo != nil {
		repoUUID, err := uuid.Parse(repo.ID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}

		return uc.repo.Update(ctx, &OCIRepoUpdateOpts{
			OCIRepoOpts: &OCIRepoOpts{
				Repository: repoURL, Username: username, Password: password, SecretName: secretName, Provider: provider,
			},
			ID: repoUUID,
		})
	}

	return uc.repo.Create(ctx, &OCIRepoCreateOpts{
		OrgID: orgUUID,
		OCIRepoOpts: &OCIRepoOpts{
			Repository: repoURL, Username: username, Password: password, SecretName: secretName, Provider: provider,
		},
	})
}

// Delete will delete the secret in the external secrets manager
// and the repository in the database
func (uc *CASBackendUseCase) Delete(ctx context.Context, id string) error {
	uc.logger.Infow("msg", "deleting OCI repository", "ID", id)

	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	repo, err := uc.repo.FindByID(ctx, repoUUID)
	if err != nil {
		return err
	} else if repo == nil {
		return NewErrNotFound("OCI repository")
	}

	uc.logger.Infow("msg", "deleting OCI repository external secrets", "ID", id, "secretName", repo.SecretName)
	// Delete the secret in the external secrets manager
	if err := uc.credsRW.DeleteCredentials(ctx, repo.SecretName); err != nil {
		return fmt.Errorf("deleting the credentials: %w", err)
	}

	uc.logger.Infow("msg", "OCI repository deleted", "ID", id)
	return uc.repo.Delete(ctx, repoUUID)
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (CASBackendValidationStatus) Values() (kinds []string) {
	for _, s := range []CASBackendValidationStatus{OCIRepoValidationOK, OCIRepoValidationFailed} {
		kinds = append(kinds, string(s))
	}

	return
}

// Validate that the repository is valid and reachable
// TODO: run this process periodically in the background
func (uc *CASBackendUseCase) PerformValidation(ctx context.Context, id string) (err error) {
	validationStatus := OCIRepoValidationFailed

	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	repo, err := uc.repo.FindByID(ctx, repoUUID)
	if err != nil {
		return err
	} else if repo == nil {
		return NewErrNotFound("OCI repository")
	}

	defer func() {
		// If the actual validation logic failed we do not update the underlying repository
		if err != nil {
			return
		}

		// Update the validation status
		uc.logger.Infow("msg", "updating validation status", "ID", id, "status", validationStatus)
		if err := uc.repo.UpdateValidationStatus(ctx, repoUUID, validationStatus); err != nil {
			uc.logger.Errorw("msg", "updating validation status", "ID", id, "error", err)
		}
	}()

	// 1 - Retrieve the credentials from the external secrets manager
	b, err := uc.ociBackendProvider.FromCredentials(ctx, repo.SecretName)
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
	validationStatus = OCIRepoValidationOK
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
