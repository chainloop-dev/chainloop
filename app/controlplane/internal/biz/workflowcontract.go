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
	"time"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type WorkflowContract struct {
	ID             uuid.UUID
	Name           string
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
	ContractBody []byte
}

type ContractUpdateOpts struct {
	Name              string
	OrgID, ContractID uuid.UUID
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

// we currently only support schema v1
func (uc *WorkflowContractUseCase) Create(ctx context.Context, orgID, name string, schema *schemav1.CraftingSchema) (*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	// If no schema is provided we create an empty one
	if schema == nil {
		schema = &schemav1.CraftingSchema{
			SchemaVersion: "v1",
		}

		if err := schema.ValidateAll(); err != nil {
			return nil, err
		}
	}

	rawSchema, err := proto.Marshal(schema)
	if err != nil {
		return nil, err
	}

	opts := &ContractCreateOpts{
		OrgID: orgUUID, Name: name,
		ContractBody: rawSchema,
	}

	return uc.repo.Create(ctx, opts)
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

	return uc.repo.FindVersionByID(ctx, versionUUID)
}

func (uc *WorkflowContractUseCase) Update(ctx context.Context, orgID, contractID, name string, schema *schemav1.CraftingSchema) (*WorkflowContractWithVersion, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, err
	}

	rawSchema, err := proto.Marshal(schema)
	if err != nil {
		return nil, err
	}

	opts := &ContractUpdateOpts{OrgID: orgUUID, ContractID: contractUUID, ContractBody: rawSchema, Name: name}

	c, err := uc.repo.Update(ctx, opts)
	if c == nil {
		return nil, NewErrNotFound("contract")
	}

	return c, err
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
		return newErrValidation(errors.New("there are associated workflows with this contract, delete them first"))
	}

	// Check that the workflow to delete belongs to the provided organization
	return uc.repo.SoftDelete(ctx, contractUUID)
}
