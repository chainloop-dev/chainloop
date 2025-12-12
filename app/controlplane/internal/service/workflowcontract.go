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

package service

import (
	"context"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowContractService struct {
	pb.UnimplementedWorkflowContractServiceServer
	*service

	contractUseCase *biz.WorkflowContractUseCase
	orgUseCase      *biz.OrganizationUseCase
	userUC          *biz.UserUseCase
}

func NewWorkflowSchemaService(uc *biz.WorkflowContractUseCase, orgUC *biz.OrganizationUseCase, userUC *biz.UserUseCase, opts ...NewOpt) *WorkflowContractService {
	return &WorkflowContractService{
		service:         newService(opts...),
		contractUseCase: uc,
		orgUseCase:      orgUC,
		userUC:          userUC,
	}
}

func (s *WorkflowContractService) List(ctx context.Context, _ *pb.WorkflowContractServiceListRequest) (*pb.WorkflowContractServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	contracts, err := s.contractUseCase.List(ctx, currentOrg.ID, biz.WithProjectFilter(s.visibleProjects(ctx)))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.WorkflowContractItem, 0, len(contracts))
	for _, s := range contracts {
		result = append(result, bizWorkFlowContractToPb(s))
	}

	return &pb.WorkflowContractServiceListResponse{Result: result}, nil
}

func (s *WorkflowContractService) Describe(ctx context.Context, req *pb.WorkflowContractServiceDescribeRequest) (*pb.WorkflowContractServiceDescribeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	contract, err := s.contractUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetName())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contract == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	// 1 - If the contract is scoped to a project, make sure the user has permission to read it
	// otherwise everyone can read it, use it
	if err := s.checkContractAccess(ctx, contract, authz.PolicyWorkflowContractRead, true); err != nil {
		return nil, err
	}

	// 2 - Get the contract version
	contractWithVersion, err := s.contractUseCase.Describe(ctx, currentOrg.ID, contract.ID.String(), int(req.GetRevision()))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contractWithVersion == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	result := &pb.WorkflowContractServiceDescribeResponse_Result{
		Contract: bizWorkFlowContractToPb(contractWithVersion.Contract),
		Revision: bizWorkFlowContractVersionToPb(contractWithVersion.Version, contractWithVersion.Contract),
	}

	return &pb.WorkflowContractServiceDescribeResponse{Result: result}, nil
}

func (s *WorkflowContractService) Create(ctx context.Context, req *pb.WorkflowContractServiceCreateRequest) (*pb.WorkflowContractServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Authorization checks
	// Force setting a project scope if RBAC is enabled
	if rbacEnabled(ctx) && !req.ProjectReference.IsSet() {
		return nil, errors.BadRequest("invalid", "project is required")
	}

	// Check organization settings for contract creation restriction
	org, err := s.orgUseCase.FindByID(ctx, currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// If setting is enabled, only org admins can create contracts (org-level or project-level)
	if org.RestrictContractCreationToOrgAdmins {
		currentUser, err := requireCurrentUser(ctx)
		if err != nil {
			return nil, err
		}

		membership, err := s.userUC.MembershipInOrg(ctx, currentUser.ID, currentOrg.Name)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		if !membership.Role.IsAdmin() {
			return nil, errors.Forbidden("forbidden", "only organization admins can create contracts")
		}
	}

	// if the project is provided we make sure it exists and the user has permission to it
	var projectID *uuid.UUID
	if req.ProjectReference.IsSet() {
		// Make sure the provided project exists and the user has permission to create contracts in it
		project, err := s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProjectReference(), authz.PolicyWorkflowContractCreate)
		if err != nil {
			return nil, err
		}

		projectID = &project.ID
	}

	// we need this token to forward it to the provider service next
	token, err := entities.GetRawToken(ctx)
	if err != nil {
		return nil, err
	}

	if len(req.RawContract) != 0 {
		if err = s.contractUseCase.ValidateContractPolicies(req.RawContract, token); err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	}

	// Validate and extract contract name
	contractName, description, err := validateAndExtractMetadata(req.RawContract, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, err
	}

	schema, err := s.contractUseCase.Create(ctx, &biz.WorkflowContractCreateOpts{
		OrgID:       currentOrg.ID,
		Name:        contractName,
		Description: description,
		RawSchema:   req.RawContract,
		ProjectID:   projectID,
	})
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowContractServiceCreateResponse{Result: bizWorkFlowContractToPb(schema)}, nil
}

func (s *WorkflowContractService) Update(ctx context.Context, req *pb.WorkflowContractServiceUpdateRequest) (*pb.WorkflowContractServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate and extract contract name
	contractName, description, err := validateAndExtractMetadata(req.RawContract, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, err
	}

	contract, err := s.contractUseCase.FindByNameInOrg(ctx, currentOrg.ID, contractName)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contract == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	if err := s.checkContractAccess(ctx, contract, authz.PolicyWorkflowContractUpdate, false); err != nil {
		return nil, err
	}

	token, err := entities.GetRawToken(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the contract policies if the raw contract is provided
	if len(req.RawContract) != 0 {
		if err = s.contractUseCase.ValidateContractPolicies(req.RawContract, token); err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	}

	schemaWithVersion, err := s.contractUseCase.Update(ctx, currentOrg.ID, contractName,
		&biz.WorkflowContractUpdateOpts{
			Description: description,
			RawSchema:   req.RawContract,
		})
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := &pb.WorkflowContractServiceUpdateResponse_Result{
		Contract: bizWorkFlowContractToPb(schemaWithVersion.Contract),
		Revision: bizWorkFlowContractVersionToPb(schemaWithVersion.Version, schemaWithVersion.Contract),
	}

	return &pb.WorkflowContractServiceUpdateResponse{Result: result}, nil
}

func (s *WorkflowContractService) Delete(ctx context.Context, req *pb.WorkflowContractServiceDeleteRequest) (*pb.WorkflowContractServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	contract, err := s.contractUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetName())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contract == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	if err := s.checkContractAccess(ctx, contract, authz.PolicyWorkflowContractDelete, false); err != nil {
		return nil, err
	}

	if err := s.contractUseCase.Delete(ctx, currentOrg.ID, contract.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowContractServiceDeleteResponse{}, nil
}

func bizWorkFlowContractToPb(schema *biz.WorkflowContract) *pb.WorkflowContractItem {
	// nolint:prealloc
	var workflowNames []string
	workflowRefs := make([]*pb.WorkflowRef, 0, len(schema.WorkflowRefs))
	for _, ref := range schema.WorkflowRefs {
		workflowRefs = append(workflowRefs, bizWorkflowRefToPb(ref))
		workflowNames = append(workflowNames, ref.Name)
	}

	result := &pb.WorkflowContractItem{
		Id:                      schema.ID.String(),
		CreatedAt:               timestamppb.New(*schema.CreatedAt),
		UpdatedAt:               timestamppb.New(*schema.UpdatedAt),
		Name:                    schema.Name,
		LatestRevision:          int32(schema.LatestRevision),
		LatestRevisionCreatedAt: timestamppb.New(*schema.LatestRevisionCreatedAt),
		WorkflowNames:           workflowNames,
		WorkflowRefs:            workflowRefs,
		//nolint:staticcheck
		Description: schema.Description,
	}

	if schema.ScopedEntity != nil {
		result.ScopedEntity = &pb.ScopedEntity{
			Type: schema.ScopedEntity.Type,
			Id:   schema.ScopedEntity.ID.String(),
			Name: schema.ScopedEntity.Name,
		}
	}

	return result
}

func bizWorkFlowContractVersionToPb(schema *biz.WorkflowContractVersion, contract *biz.WorkflowContract) *pb.WorkflowContractVersionItem {
	formatTranslator := func(unmarshal.RawFormat) pb.WorkflowContractVersionItem_RawBody_Format {
		switch schema.Schema.Format {
		case unmarshal.RawFormatJSON:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_JSON
		case unmarshal.RawFormatYAML:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML
		case unmarshal.RawFormatCUE:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_CUE
		}

		return pb.WorkflowContractVersionItem_RawBody_FORMAT_UNSPECIFIED
	}

	item := &pb.WorkflowContractVersionItem{
		Id:        schema.ID.String(),
		CreatedAt: timestamppb.New(*schema.CreatedAt),
		Revision:  int32(schema.Revision),
		Contract: &pb.WorkflowContractVersionItem_V1{
			V1: schema.Schema.Schema,
		},
		RawContract: &pb.WorkflowContractVersionItem_RawBody{
			Body:   schema.Schema.Raw,
			Format: formatTranslator(schema.Schema.Format),
		},
	}

	// Standardize the API to always provide description in the version.
	// Add description from the proper source (new schemav2 or legacy top level contract)
	if schema.Schema.Schemav2 != nil {
		item.Description = schema.Schema.Schemav2.GetMetadata().GetDescription()
	} else {
		//nolint:staticcheck
		item.Description = contract.Description
	}

	return item
}

// checkContractAccess checks if the current user can manage a contract
// if the contract is global it makes sure that the user is an admin
// if the contract is scoped to a project it makes sure that the user has permission in the project
func (s *WorkflowContractService) checkContractAccess(ctx context.Context, contract *biz.WorkflowContract, policy *authz.Policy, allowGlobalAccess bool) error {
	// 1 - Only admins can manage global contracts unless allowGlobalAccess is true
	if contract.IsGlobalScoped() && rbacEnabled(ctx) && !allowGlobalAccess {
		return errors.BadRequest("invalid", "you can not manage a global contract")
	}

	// 2 - If the contract is scoped to a project, make sure the user has permission to read it
	if contract.IsProjectScoped() {
		if err := s.authorizeResource(ctx, policy, authz.ResourceTypeProject, contract.ScopedEntity.ID); err != nil {
			return err
		}
	}

	return nil
}

// validateAndExtractName validates and extracts a name from either
// an explicit name parameter OR from metadata.name in the v2 contract.
// Returns error when:
// - Neither explicit name nor metadata.name is provided
// - Both are provided but have different values
func validateAndExtractMetadata(rawContract []byte, explicitName, explicitDesc string) (string, *string, error) {
	var name, desc string

	// Extract name from v2 metadata if present
	metadata, err := extractMetadata(rawContract)
	if err != nil {
		return "", nil, err
	}

	if metadata != nil {
		// Both provided
		if explicitName != "" && metadata.GetName() != "" {
			// If both are provided but different, that's a conflict
			if explicitName != metadata.GetName() {
				return "", nil, errors.BadRequest("invalid", "contract name cannot be changed")
			}
		}

		name = metadata.GetName()
		desc = metadata.GetDescription()
	} else {
		// Neither provided
		if explicitName == "" {
			return "", nil, errors.BadRequest("invalid", "name is required: either provide explicit name or include metadata.name in the schema")
		}

		name = explicitName
		desc = explicitDesc
	}

	// description is optional, make sure we return a pointer or nil
	var description *string
	if desc != "" {
		description = &desc
	}
	return name, description, nil
}

// extractNameFromMetadata attempts to extract the name from metadata.name in v2 contracts
func extractMetadata(rawContract []byte) (*schemav1.Metadata, error) {
	if len(rawContract) == 0 {
		return nil, nil
	}

	// Identify the format
	format, err := unmarshal.IdentifyFormat(rawContract)
	if err != nil {
		return nil, errors.BadRequest("invalid", "failed to identify contract format")
	}

	// Try parsing as v2 Contract
	v2Contract := &schemav1.CraftingSchemaV2{}
	if err := unmarshal.FromRaw(rawContract, format, v2Contract, true); err == nil {
		if v2Contract.GetMetadata() != nil {
			return v2Contract.GetMetadata(), nil
		}
	}

	// If v2 parsing failed or no metadata, return nothing
	return nil, nil
}
