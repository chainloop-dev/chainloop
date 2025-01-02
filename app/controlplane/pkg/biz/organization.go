//
// Copyright 2024 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/moby/moby/pkg/namesgenerator"
)

type Organization struct {
	ID, Name  string
	CreatedAt *time.Time
	// BlockOnPolicyViolation blocks the workflow run if policy evaluation fails
	BlockOnPolicyViolation bool
}

type OrganizationRepo interface {
	FindByID(ctx context.Context, orgID uuid.UUID) (*Organization, error)
	FindByName(ctx context.Context, name string) (*Organization, error)
	Create(ctx context.Context, name string) (*Organization, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type OrganizationUseCase struct {
	orgRepo           OrganizationRepo
	logger            *log.Helper
	casBackendUseCase *CASBackendUseCase
	integrationUC     *IntegrationUseCase
	membershipRepo    MembershipRepo
	onboardingConfig  []*config.OnboardingSpec
	auditor           *AuditorUseCase
}

func NewOrganizationUseCase(repo OrganizationRepo, repoUC *CASBackendUseCase, auditor *AuditorUseCase, iUC *IntegrationUseCase, mRepo MembershipRepo, onboardingConfig []*config.OnboardingSpec, l log.Logger) *OrganizationUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &OrganizationUseCase{orgRepo: repo,
		logger:            servicelogger.ScopedHelper(l, "biz/organization"),
		casBackendUseCase: repoUC,
		integrationUC:     iUC,
		membershipRepo:    mRepo,
		onboardingConfig:  onboardingConfig,
		auditor:           auditor,
	}
}

const RandomNameMaxTries = 10

type createOptions struct {
	createInlineBackend bool
}

type CreateOpt func(*createOptions)

// Optionally create an inline CAS-backend
func WithCreateInlineBackend() CreateOpt {
	return func(o *createOptions) {
		o.createInlineBackend = true
	}
}

func (uc *OrganizationUseCase) CreateWithRandomName(ctx context.Context, opts ...CreateOpt) (*Organization, error) {
	// Try 10 times to create a random name
	for i := 0; i < RandomNameMaxTries; i++ {
		// Create a random name
		prefix := namesgenerator.GetRandomName(0)
		name, err := generateValidDNS1123WithSuffix(prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random name: %w", err)
		}

		org, err := uc.doCreate(ctx, name, opts...)
		if err != nil {
			// We retry if the organization already exists
			if IsErrAlreadyExists(err) {
				uc.logger.Debugw("msg", "Org exists!", "name", name)
				continue
			}
			uc.logger.Debugw("msg", "BOOM", "name", name, "err", err)
			return nil, err
		}

		return org, nil
	}

	return nil, errors.New("failed to create a random organization name")
}

// Create an organization with the given name
func (uc *OrganizationUseCase) Create(ctx context.Context, name string, opts ...CreateOpt) (*Organization, error) {
	org, err := uc.doCreate(ctx, name, opts...)
	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrAlreadyExistsStr("organization already exists")
		}

		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

var errOrgName = errors.New("org names must only contain lowercase letters, numbers, or hyphens. Examples of valid org names are \"myorg\", \"myorg-123\"")

func (uc *OrganizationUseCase) doCreate(ctx context.Context, name string, opts ...CreateOpt) (*Organization, error) {
	uc.logger.Infow("msg", "Creating organization", "name", name)

	if err := ValidateIsDNS1123(name); err != nil {
		return nil, NewErrValidation(errOrgName)
	}

	options := &createOptions{}
	for _, o := range opts {
		o(options)
	}

	org, err := uc.orgRepo.Create(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	if options.createInlineBackend {
		// Create inline CAS-backend
		if _, err := uc.casBackendUseCase.CreateInlineFallbackBackend(ctx, org.ID); err != nil {
			return nil, fmt.Errorf("failed to create fallback backend: %w", err)
		}
	}

	orgUUID, err := uuid.Parse(org.ID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	uc.auditor.Dispatch(ctx, &events.OrgCreated{
		OrgBase: &events.OrgBase{OrgID: &orgUUID, OrgName: org.Name}}, &orgUUID,
	)

	return org, nil
}

func (uc *OrganizationUseCase) FindByID(ctx context.Context, id string) (*Organization, error) {
	orgUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	org, err := uc.orgRepo.FindByID(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find organization: %w", err)
	} else if org == nil {
		return nil, NewErrNotFound("organization")
	}

	return org, nil
}

// Delete deletes an organization and all relevant data
// This includes:
// - The organization
// - The associated repositories
// - The associated integrations
// The reason for just deleting these two associated components only is because
// they have external secrets that need to be deleted as well, and for that we leverage their own delete methods
// The rest of the data gets removed by the database cascade delete
func (uc *OrganizationUseCase) Delete(ctx context.Context, id string) error {
	orgUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	org, err := uc.orgRepo.FindByID(ctx, orgUUID)
	if err != nil {
		return err
	} else if org == nil {
		return NewErrNotFound("organization")
	}

	// Delete all the integrations
	integrations, err := uc.integrationUC.List(ctx, id)
	if err != nil {
		return err
	}

	for _, i := range integrations {
		if err := uc.integrationUC.Delete(ctx, id, i.ID.String()); err != nil {
			return err
		}
	}

	backends, err := uc.casBackendUseCase.List(ctx, org.ID)
	if err != nil {
		return fmt.Errorf("failed to list backends: %w", err)
	}

	for _, b := range backends {
		if err := uc.casBackendUseCase.Delete(ctx, b.ID.String()); err != nil {
			return fmt.Errorf("failed to delete backend: %w", err)
		}
	}

	// Delete the organization
	return uc.orgRepo.Delete(ctx, orgUUID)
}

// AutoOnboardOrganizations creates the organizations specified in the onboarding config and assigns the user to them
// with the specified role if they are not already a member.
func (uc *OrganizationUseCase) AutoOnboardOrganizations(ctx context.Context, userID string) error {
	// Parse user UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	for _, spec := range uc.onboardingConfig {
		// Find organization or skip onboarding if it doesn't exist
		org, err := uc.orgRepo.FindByName(ctx, spec.GetName())
		if err != nil {
			return fmt.Errorf("failed to find organization: %w", err)
		} else if org == nil {
			uc.logger.Infow("msg", "Organization not found", "name", spec.GetName())
			continue
		}

		// Parse organization UUID
		orgUUID, err := uuid.Parse(org.ID)
		if err != nil {
			return NewErrInvalidUUID(err)
		}

		// Ensure user membership
		if err := uc.ensureUserMembership(ctx, orgUUID, userUUID, PbRoleToBiz(spec.GetRole())); err != nil {
			return fmt.Errorf("failed to ensure user membership: %w", err)
		}
	}

	return nil
}

// ensureUserMembership ensures that a user is a member of the specified organization with the appropriate role.
// If the membership does not exist, it creates it.
func (uc *OrganizationUseCase) ensureUserMembership(ctx context.Context, orgUUID, userUUID uuid.UUID, role authz.Role) error {
	m, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgUUID, userUUID)
	if err != nil {
		return fmt.Errorf("failed to find membership: %w", err)
	}

	if m != nil {
		// Membership already exists, no further action needed
		return nil
	}

	// Create the membership with the specified role
	_, err = uc.membershipRepo.Create(ctx, orgUUID, userUUID, true, role)
	if err != nil {
		return fmt.Errorf("failed to create membership: %w", err)
	}

	uc.logger.Infow("msg", "User auto-onboarded to organization", "org", orgUUID, "user", userUUID, "role", role)
	return nil
}
