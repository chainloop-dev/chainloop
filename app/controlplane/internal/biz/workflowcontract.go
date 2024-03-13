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
	"time"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type WorkflowContract struct {
	ID             uuid.UUID
	Name           string
	Description    string
	LatestRevision int
	CreatedAt      *time.Time
	// WorkflowIDs is the list of workflows associated with this contract
	WorkflowIDs []string
}

type WorkflowContractVersion struct {
	ID        uuid.UUID
	Revision  int
	CreatedAt *time.Time
	BodyV1    *schemav1.CraftingSchema
}

type WorkflowContractWithVersion struct {
	Contract *WorkflowContract
	Version  *WorkflowContractVersion
}

type WorkflowContractRepo interface {
	Create(ctx context.Context, opts *ContractCreateOpts) (*WorkflowContract, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*WorkflowContract, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*WorkflowContract, error)
	Describe(ctx context.Context, orgID, contractID uuid.UUID, revision int) (*WorkflowContractWithVersion, error)
	FindVersionByID(ctx context.Context, versionID uuid.UUID) (*WorkflowContractVersion, error)
	Update(ctx context.Context, opts *ContractUpdateOpts) (*WorkflowContractWithVersion, error)
	SoftDelete(ctx context.Context, contractID uuid.UUID) error
}

type ContractCreateOpts struct {
	Name         string
	OrgID        uuid.UUID
	Description  *string
	ContractBody []byte
}

type ContractUpdateOpts struct {
	Name              string
	OrgID, ContractID uuid.UUID
	Description       *string
	ContractBody      []byte
}

type WorkflowContractUseCase struct {
	repo   WorkflowContractRepo
	logger *log.Helper
}

func NewWorkflowContractUseCase(repo WorkflowContractRepo, logger log.Logger) *WorkflowContractUseCase {
	return &WorkflowContractUseCase{repo: repo, logger: log.NewHelper(logger)}
}

func (uc *WorkflowContractUseCase) List(ctx context.Context, orgID string) ([]*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	return uc.repo.List(ctx, orgUUID)
}

func (uc *WorkflowContractUseCase) FindByIDInOrg(ctx context.Context, orgID, contractID string) (*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, err
	}

	return uc.repo.FindByIDInOrg(ctx, orgUUID, contractUUID)
}

type WorkflowContractCreateOpts struct {
	OrgID, Name string
	Schema      *schemav1.CraftingSchema
	Description *string
	// Make sure that the name is unique in the organization
	AddUniquePrefix bool
}

// we currently only support schema v1
func (uc *WorkflowContractUseCase) Create(ctx context.Context, opts *WorkflowContractCreateOpts) (*WorkflowContract, error) {
	if opts.OrgID == "" || opts.Name == "" {
		return nil, NewErrValidationStr("organization and name are required")
	}

	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, err
	}

	if err := ValidateIsDNS1123(opts.Name); err != nil {
		return nil, NewErrValidation(err)
	}

	// If no schema is provided we create an empty one
	if opts.Schema == nil {
		opts.Schema = &schemav1.CraftingSchema{
			SchemaVersion: "v1",
		}
	}

	if err := opts.Schema.ValidateAll(); err != nil {
		return nil, err
	}

	rawSchema, err := proto.Marshal(opts.Schema)
	if err != nil {
		return nil, err
	}

	// Create a workflow with a unique name if needed
	args := &ContractCreateOpts{
		OrgID: orgUUID, Name: opts.Name, Description: opts.Description,
		ContractBody: rawSchema,
	}

	var c *WorkflowContract
	if opts.AddUniquePrefix {
		c, err = uc.createWithUniqueName(ctx, args)
	} else {
		c, err = uc.repo.Create(ctx, args)
	}

	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("name already taken")
		}

		return nil, fmt.Errorf("failed to create contract: %w", err)
	}

	return c, nil
}

func (uc *WorkflowContractUseCase) createWithUniqueName(ctx context.Context, opts *ContractCreateOpts) (*WorkflowContract, error) {
	originalName := opts.Name

	for i := 0; i < RandomNameMaxTries; i++ {
		// append a suffix
		if i > 0 {
			var err error
			opts.Name, err = generateValidDNS1123WithSuffix(originalName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate random name: %w", err)
			}
		}

		c, err := uc.repo.Create(ctx, opts)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				continue
			}

			return nil, fmt.Errorf("failed to create contract: %w", err)
		}

		return c, nil
	}

	return nil, NewErrValidationStr("name already taken")
}

func (uc *WorkflowContractUseCase) Describe(ctx context.Context, orgID, contractID string, revision int) (*WorkflowContractWithVersion, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, err
	}

	return uc.repo.Describe(ctx, orgUUID, contractUUID, revision)
}

func (uc *WorkflowContractUseCase) FindVersionByID(ctx context.Context, versionID string) (*WorkflowContractVersion, error) {
	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		return nil, err
	}

	r, err := uc.repo.FindVersionByID(ctx, versionUUID)
	if err != nil {
		return nil, fmt.Errorf("finding contract version: %w", err)
	} else if r == nil {
		return nil, NewErrNotFound("contract version")
	}

	return r, nil
}

type WorkflowContractUpdateOpts struct {
	Name        string
	Schema      *schemav1.CraftingSchema
	Description *string
}

func (uc *WorkflowContractUseCase) Update(ctx context.Context, orgID, contractID string, opts *WorkflowContractUpdateOpts) (*WorkflowContractWithVersion, error) {
	if opts == nil {
		return nil, NewErrValidationStr("no updates provided")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, err
	}

	if opts.Name != "" {
		if err := ValidateIsDNS1123(opts.Name); err != nil {
			return nil, NewErrValidation(err)
		}
	}

	rawSchema, err := proto.Marshal(opts.Schema)
	if err != nil {
		return nil, err
	}

	args := &ContractUpdateOpts{OrgID: orgUUID, ContractID: contractUUID, ContractBody: rawSchema, Name: opts.Name, Description: opts.Description}

	c, err := uc.repo.Update(ctx, args)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("name already taken")
		}

		return nil, fmt.Errorf("failed to update contract: %w", err)
	} else if c == nil {
		return nil, NewErrNotFound("contract")
	}

	return c, nil
}

// Delete soft-deletes the entry
func (uc *WorkflowContractUseCase) Delete(ctx context.Context, orgID, contractID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return err
	}

	// Make sure that the contract is from this org and it has not associated workflows
	contract, err := uc.repo.FindByIDInOrg(ctx, orgUUID, contractUUID)
	if err != nil {
		return err
	}

	if contract == nil {
		return NewErrNotFound("contract")
	}

	if len(contract.WorkflowIDs) > 0 {
		return NewErrValidation(errors.New("there are associated workflows with this contract, delete them first"))
	}

	// Check that the workflow to delete belongs to the provided organization
	return uc.repo.SoftDelete(ctx, contractUUID)
}
