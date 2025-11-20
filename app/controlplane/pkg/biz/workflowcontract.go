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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/protoyaml-go"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/policies"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	loader "github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

type WorkflowContract struct {
	ID                      uuid.UUID
	Name                    string
	Description             string
	LatestRevision          int
	LatestRevisionCreatedAt *time.Time
	CreatedAt               *time.Time
	UpdatedAt               *time.Time
	// WorkflowRefs is the list of workflows associated with this contract
	WorkflowRefs []*WorkflowRef
	// entity the contract is scoped to, if not set it's scoped to the organization
	ScopedEntity *ScopedEntity
}

type ScopedEntity struct {
	// Type is the type of the scoped entity i.e project or org
	Type string
	// ID is the id of the scoped entity
	ID uuid.UUID
	// Name is the name of the scoped entity
	Name string
}

type WorkflowContractVersion struct {
	ID        uuid.UUID
	Revision  int
	CreatedAt *time.Time
	Schema    *Contract
}

type Contract struct {
	// Raw representation of the contract in yaml, json, or cue
	// it maintain the format provided by the user
	Raw []byte
	// Detected format as provided by the user
	Format unmarshal.RawFormat
	// marshalled proto v1 contract
	Schema *schemav1.CraftingSchema
	// marshalled proto v2 contract
	Schemav2 *schemav1.CraftingSchemaV2
}

// isV2Schema returns true if the contract uses the v2 CraftingSchema format
func (c *Contract) isV2Schema() bool {
	return c.Schemav2 != nil
}

// isV1Schema returns true if the contract uses the v1 CraftingSchema format
func (c *Contract) isV1Schema() bool {
	return c.Schema != nil
}

type WorkflowContractWithVersion struct {
	Contract *WorkflowContract
	Version  *WorkflowContractVersion
}

type WorkflowContractRepo interface {
	Create(ctx context.Context, opts *ContractCreateOpts) (*WorkflowContract, error)
	List(ctx context.Context, orgID uuid.UUID, filter *WorkflowContractListFilters) ([]*WorkflowContract, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*WorkflowContract, error)
	FindByNameInOrg(ctx context.Context, orgID uuid.UUID, name string) (*WorkflowContract, error)
	Describe(ctx context.Context, orgID, contractID uuid.UUID, revision int, opts ...ContractQueryOpt) (*WorkflowContractWithVersion, error)
	FindVersionByID(ctx context.Context, versionID uuid.UUID) (*WorkflowContractWithVersion, error)
	Update(ctx context.Context, orgID uuid.UUID, name string, opts *ContractUpdateOpts) (*WorkflowContractWithVersion, error)
	SoftDelete(ctx context.Context, contractID uuid.UUID) error
}

type ContractQueryOpts struct {
	// SkipGetReferences will skip the get references subquery
	// The references are composed by the project name and workflow name
	SkipGetReferences bool
}

type ContractQueryOpt func(opts *ContractQueryOpts)

func WithoutReferences() ContractQueryOpt {
	return func(opts *ContractQueryOpts) {
		opts.SkipGetReferences = true
	}
}

type ContractCreateOpts struct {
	Name        string
	OrgID       uuid.UUID
	Description *string
	// raw representation of the contract in whatever original format it was (json, yaml, ...)
	Contract *Contract
	// ProjectID indicates the project to be scoped to
	ProjectID *uuid.UUID
}

type ContractUpdateOpts struct {
	Description *string
	// raw representation of the contract in whatever original format it was (json, yaml, ...)
	Contract *Contract
}

type WorkflowContractUseCase struct {
	repo           WorkflowContractRepo
	logger         *log.Helper
	policyRegistry *policies.Registry
	auditorUC      *AuditorUseCase
}

func NewWorkflowContractUseCase(repo WorkflowContractRepo, policyRegistry *policies.Registry, auditorUC *AuditorUseCase, logger log.Logger) *WorkflowContractUseCase {
	return &WorkflowContractUseCase{repo: repo, policyRegistry: policyRegistry, auditorUC: auditorUC, logger: log.NewHelper(logger)}
}

type WorkflowContractListFilters struct {
	// FilterByProjects is used to filter the result by a project list
	// If it's empty, no filter will be applied
	FilterByProjects []uuid.UUID
}

type WorkflowListOpt func(opts *WorkflowContractListFilters)

func WithProjectFilter(projectIDs []uuid.UUID) WorkflowListOpt {
	return func(opts *WorkflowContractListFilters) {
		opts.FilterByProjects = projectIDs
	}
}

func (uc *WorkflowContractUseCase) List(ctx context.Context, orgID string, opts ...WorkflowListOpt) ([]*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	filters := &WorkflowContractListFilters{}
	for _, opt := range opts {
		opt(filters)
	}

	return uc.repo.List(ctx, orgUUID, filters)
}

func (uc *WorkflowContractUseCase) FindByIDInOrg(ctx context.Context, orgID, contractID string) (*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindByIDInOrg(ctx, orgUUID, contractUUID)
}

func (uc *WorkflowContractUseCase) FindByNameInOrg(ctx context.Context, orgID, name string) (*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindByNameInOrg(ctx, orgUUID, name)
}

func (c *WorkflowContract) IsGlobalScoped() bool {
	return c.ScopedEntity == nil
}

func (c *WorkflowContract) IsProjectScoped() bool {
	return c.ScopedEntity != nil && c.ScopedEntity.Type == string(ContractScopeProject)
}

type WorkflowContractCreateOpts struct {
	OrgID, Name string
	RawSchema   []byte
	Description *string
	ProjectID   *uuid.UUID
	// Make sure that the name is unique in the organization
	AddUniquePrefix bool
}

// createDefaultContract creates a new default contract with the specified name
func createDefaultContract(name string) (*Contract, error) {
	defaultSchema := &schemav1.CraftingSchemaV2{
		ApiVersion: "chainloop.dev/v1",
		Kind:       "Contract",
		Metadata:   &schemav1.Metadata{Name: name},
		Spec:       &schemav1.CraftingSchemaV2Spec{},
	}

	// Marshal to YAML using protoyaml
	rawYAML, err := protoyaml.Marshal(defaultSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default contract to YAML: %w", err)
	}

	// Convert to v1 for backward compatibility with old CLIs
	v1Schema := defaultSchema.ToV1()
	return &Contract{
		Raw:      rawYAML,
		Format:   unmarshal.RawFormatYAML,
		Schema:   v1Schema,
		Schemav2: defaultSchema,
	}, nil
}

func (uc *WorkflowContractUseCase) Create(ctx context.Context, opts *WorkflowContractCreateOpts) (*WorkflowContract, error) {
	if opts.OrgID == "" {
		return nil, NewErrValidationStr("organization is required")
	}

	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, err
	}

	if opts.Name == "" {
		return nil, NewErrValidationStr("name is required")
	}

	if err := ValidateIsDNS1123(opts.Name); err != nil {
		return nil, NewErrValidation(err)
	}

	var contract *Contract
	if len(opts.RawSchema) > 0 {
		// Load the provided contract
		c, err := identifyUnMarshalAndValidateRawContract(opts.RawSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to load contract: %w", err)
		}
		contract = c
	} else {
		// Use default contract with the user-provided name
		c, err := createDefaultContract(opts.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create default contract: %w", err)
		}
		contract = c
	}

	// Create a workflow with a unique name if needed
	args := &ContractCreateOpts{
		OrgID:       orgUUID,
		Name:        opts.Name,
		Description: opts.Description,
		Contract:    contract,
		ProjectID:   opts.ProjectID,
	}

	var c *WorkflowContract
	if opts.AddUniquePrefix {
		c, err = uc.createWithUniqueName(ctx, args)
	} else {
		c, err = uc.repo.Create(ctx, args)
	}

	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrAlreadyExistsStr("name already taken")
		}

		return nil, fmt.Errorf("failed to create contract: %w", err)
	}

	// Dispatch the event
	uc.auditorUC.Dispatch(ctx, &events.WorkflowContractCreated{
		WorkflowContractBase: &events.WorkflowContractBase{
			WorkflowContractID:   &c.ID,
			WorkflowContractName: c.Name,
		},
	}, &orgUUID)

	return c, nil
}

func (uc *WorkflowContractUseCase) createWithUniqueName(ctx context.Context, opts *ContractCreateOpts) (*WorkflowContract, error) {
	originalName := opts.Name

	for i := 0; i < RandomNameMaxTries; i++ {
		// append a suffiEmptyDefaultContractx
		if i > 0 {
			var err error
			opts.Name, err = generateValidDNS1123WithSuffix(originalName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate random name: %w", err)
			}
		}

		c, err := uc.repo.Create(ctx, opts)
		if err != nil {
			if IsErrAlreadyExists(err) {
				continue
			}

			return nil, fmt.Errorf("failed to create contract: %w", err)
		}

		return c, nil
	}

	return nil, NewErrValidationStr("name already taken")
}

func (uc *WorkflowContractUseCase) Describe(ctx context.Context, orgID, contractID string, revision int, opts ...ContractQueryOpt) (*WorkflowContractWithVersion, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(contractID)
	if err != nil {
		return nil, err
	}

	return uc.repo.Describe(ctx, orgUUID, contractUUID, revision, opts...)
}

func (uc *WorkflowContractUseCase) FindVersionByID(ctx context.Context, versionID string) (*WorkflowContractWithVersion, error) {
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
	RawSchema   []byte
	Description *string
}

func (uc *WorkflowContractUseCase) Update(ctx context.Context, orgID, name string, opts *WorkflowContractUpdateOpts) (*WorkflowContractWithVersion, error) {
	if opts == nil {
		return nil, NewErrValidationStr("no updates provided")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	var contract *Contract
	if len(opts.RawSchema) > 0 {
		c, err := identifyUnMarshalAndValidateRawContract(opts.RawSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to load contract: %w", err)
		}

		contract = c
	}

	wfContractPreUpdate, err := uc.repo.FindByNameInOrg(ctx, orgUUID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find contract %s in org %s: %w", name, orgUUID, err)
	}

	args := &ContractUpdateOpts{Description: opts.Description, Contract: contract}
	c, err := uc.repo.Update(ctx, orgUUID, name, args)
	if err != nil {
		return nil, fmt.Errorf("failed to update contract: %w", err)
	} else if c == nil {
		return nil, NewErrNotFound("contract")
	}

	// Dispatch the event
	eventPayload := &events.WorkflowContractUpdated{
		WorkflowContractBase: &events.WorkflowContractBase{
			WorkflowContractID:   &c.Contract.ID,
			WorkflowContractName: c.Contract.Name,
		},
		NewDescription: opts.Description,
	}

	// Check if the revisions have changed
	if wfContractPreUpdate.LatestRevision != c.Version.Revision {
		eventPayload.NewRevision = &c.Version.Revision
		eventPayload.NewRevisionID = &c.Version.ID
	}

	uc.auditorUC.Dispatch(ctx, eventPayload, &orgUUID)

	return c, nil
}

func (uc *WorkflowContractUseCase) ValidateContractPolicies(rawSchema []byte, token string) error {
	// Validate that externally provided policies exist
	c, err := identifyUnMarshalAndValidateRawContract(rawSchema)
	if err != nil {
		return NewErrValidation(err)
	}

	switch {
	case c.isV1Schema():
		// Handle v1 schema
		// DEPRECATED: v1 schema is deprecated, use v2 Contract format instead
		schema := c.Schema
		for _, att := range schema.GetPolicies().GetAttestation() {
			if _, err := uc.findAndValidatePolicy(att, token); err != nil {
				return NewErrValidation(err)
			}
		}
		for _, att := range schema.GetPolicies().GetMaterials() {
			if _, err := uc.findAndValidatePolicy(att, token); err != nil {
				return NewErrValidation(err)
			}
		}
		for _, gatt := range schema.GetPolicyGroups() {
			if _, err := uc.findAndValidatePolicyGroup(gatt, token); err != nil {
				return NewErrValidation(err)
			}
		}
	case c.isV2Schema():
		// Handle v2 schema
		spec := c.Schemav2.GetSpec()
		if spec.GetPolicies() != nil {
			for _, att := range spec.GetPolicies().GetAttestation() {
				if _, err := uc.findAndValidatePolicy(att, token); err != nil {
					return NewErrValidation(err)
				}
			}
			for _, att := range spec.GetPolicies().GetMaterials() {
				if _, err := uc.findAndValidatePolicy(att, token); err != nil {
					return NewErrValidation(err)
				}
			}
		}
		for _, gatt := range spec.GetPolicyGroups() {
			if _, err := uc.findAndValidatePolicyGroup(gatt, token); err != nil {
				return NewErrValidation(err)
			}
		}
	default:
		return NewErrValidation(fmt.Errorf("invalid schema format"))
	}

	return nil
}

func (uc *WorkflowContractUseCase) ValidatePolicyAttachment(providerName string, att *schemav1.PolicyAttachment, token string) error {
	provider, err := uc.findProvider(providerName)
	if err != nil {
		return err
	}

	if err = provider.ValidateAttachment(att, token); err != nil {
		return fmt.Errorf("invalid attachment: %w", err)
	}

	return nil
}

func (uc *WorkflowContractUseCase) findAndValidatePolicy(att *schemav1.PolicyAttachment, token string) (*schemav1.Policy, error) {
	var policy *schemav1.Policy

	if att.GetEmbedded() != nil {
		policy = att.GetEmbedded()
	}

	// if it should come from a provider, check that it's available
	// [chainloop://][provider:][org_name/]name
	if loader.IsProviderScheme(att.GetRef()) {
		pr := loader.ProviderParts(att.GetRef())
		// Validate attachment
		if err := uc.ValidatePolicyAttachment(pr.Provider, att, token); err != nil {
			return nil, err
		}

		remotePolicy, err := uc.GetPolicy(pr.Provider, pr.Name, pr.OrgName, "", token)
		if err != nil {
			return nil, err
		}
		policy = remotePolicy.Policy
	}

	if policy != nil {
		// validate policy arguments
		with := att.GetWith()
		for _, input := range policy.GetSpec().GetInputs() {
			_, ok := with[input.GetName()]
			if !ok && input.GetRequired() {
				return nil, NewErrValidation(fmt.Errorf("missing required input %q", input.GetName()))
			}
		}
	}

	// return policy or nil, as it might not be available in this context
	return policy, nil
}

func (uc *WorkflowContractUseCase) findAndValidatePolicyGroup(att *schemav1.PolicyGroupAttachment, token string) (*schemav1.PolicyGroup, error) {
	if !loader.IsProviderScheme(att.GetRef()) {
		// Otherwise, don't return an error, as it might consist of a local policy, not available in this context
		return nil, nil
	}

	// if it should come from a provider, check that it's available
	// [chainloop://][provider/]name
	pr := loader.ProviderParts(att.GetRef())
	remoteGroup, err := uc.GetPolicyGroup(pr.Provider, pr.Name, pr.OrgName, "", token)
	if err != nil {
		return nil, NewErrValidation(fmt.Errorf("failed to get policy group: %w", err))
	}

	if remoteGroup.PolicyGroup != nil {
		// validate group arguments
		with := att.GetWith()
		for _, input := range remoteGroup.PolicyGroup.GetSpec().GetInputs() {
			_, ok := with[input.GetName()]
			if !ok && input.GetRequired() {
				return nil, NewErrValidation(fmt.Errorf("missing required input %q for group", input.GetName()))
			}

			if input.GetRequired() && input.GetDefault() != "" {
				return nil, NewErrValidation(fmt.Errorf("input %s can not be required and have a default at the same time", input.GetName()))
			}
		}
	}

	// Validate skip list
	if err := uc.validateSkipList(remoteGroup.PolicyGroup, att, token); err != nil {
		return nil, fmt.Errorf("failed to validate skip list: %w", err)
	}

	return remoteGroup.PolicyGroup, nil
}

// validateSkipList checks if policy names in the skip list exist in the group
// and returns an error if any unknown policy names are found
func (uc *WorkflowContractUseCase) validateSkipList(group *schemav1.PolicyGroup, groupAtt *schemav1.PolicyGroupAttachment, token string) error {
	if len(groupAtt.GetSkip()) == 0 {
		return nil
	}

	// Collect all policy names in the group
	policyNames := make(map[string]bool)
	policies := group.GetSpec().GetPolicies()

	// Collect material policy names
	for _, groupMaterial := range policies.GetMaterials() {
		for _, policyAtt := range groupMaterial.GetPolicies() {
			policy, err := uc.findAndValidatePolicy(policyAtt, token)
			if err != nil {
				return fmt.Errorf("failed to get policy name during skip list validation: %w", err)
			}
			policyNames[policy.GetMetadata().GetName()] = true
		}
	}

	// Collect attestation policy names
	for _, policyAtt := range policies.GetAttestation() {
		policy, err := uc.findAndValidatePolicy(policyAtt, token)
		if err != nil {
			return fmt.Errorf("failed to get policy name during skip list validation: %w", err)
		}
		policyNames[policy.GetMetadata().GetName()] = true
	}

	// Check each skip entry against collected policy names and collect unknown ones
	var unknownPolicies []string
	for _, skipName := range groupAtt.GetSkip() {
		if !policyNames[skipName] {
			unknownPolicies = append(unknownPolicies, skipName)
		}
	}

	// Return error if there are unknown policies
	if len(unknownPolicies) > 0 {
		return fmt.Errorf("policy %q not found in policy group %q", strings.Join(unknownPolicies, ", "), group.GetMetadata().GetName())
	}

	return nil
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

	if len(contract.WorkflowRefs) > 0 {
		return NewErrValidation(errors.New("there are associated workflows with this contract, delete them first"))
	}

	// Check that the workflow to delete belongs to the provided organization
	if err := uc.repo.SoftDelete(ctx, contractUUID); err != nil {
		return fmt.Errorf("failed to delete contract: %w", err)
	}

	// Dispatch the event
	uc.auditorUC.Dispatch(ctx, &events.WorkflowContractDeleted{
		WorkflowContractBase: &events.WorkflowContractBase{
			WorkflowContractID:   &contract.ID,
			WorkflowContractName: contract.Name,
		},
	}, &orgUUID)

	return nil
}

type RemotePolicy struct {
	ProviderRef *policies.PolicyReference
	Policy      *schemav1.Policy
}

type RemotePolicyGroup struct {
	ProviderRef *policies.PolicyReference
	PolicyGroup *schemav1.PolicyGroup
}

// GetPolicy retrieves a policy from a policy provider
func (uc *WorkflowContractUseCase) GetPolicy(providerName, policyName, policyOrgName, currentOrgName, token string) (*RemotePolicy, error) {
	provider, err := uc.findProvider(providerName)
	if err != nil {
		return nil, err
	}

	policy, ref, err := provider.Resolve(policyName, policyOrgName, policies.ProviderAuthOpts{
		Token:   token,
		OrgName: currentOrgName,
	})
	if err != nil {
		if errors.Is(err, policies.ErrNotFound) {
			return nil, NewErrNotFound(fmt.Sprintf("policy %q", policyName))
		}

		return nil, fmt.Errorf("failed to resolve policy: %w. Available providers: %s", err, uc.policyRegistry.GetProviderNames())
	}

	return &RemotePolicy{Policy: policy, ProviderRef: ref}, nil
}

func (uc *WorkflowContractUseCase) GetPolicyGroup(providerName, groupName, groupOrgName, currentOrgName, token string) (*RemotePolicyGroup, error) {
	provider, err := uc.findProvider(providerName)
	if err != nil {
		return nil, err
	}

	group, ref, err := provider.ResolveGroup(groupName, groupOrgName, policies.ProviderAuthOpts{
		Token:   token,
		OrgName: currentOrgName,
	})
	if err != nil {
		if errors.Is(err, policies.ErrNotFound) {
			return nil, NewErrNotFound(fmt.Sprintf("policy group %q", groupName))
		}

		return nil, fmt.Errorf("failed to resolve policy: %w. Available providers: %s", err, uc.policyRegistry.GetProviderNames())
	}

	return &RemotePolicyGroup{PolicyGroup: group, ProviderRef: ref}, nil
}

func (uc *WorkflowContractUseCase) findProvider(providerName string) (*policies.PolicyProvider, error) {
	if len(uc.policyRegistry.GetProviderNames()) == 0 {
		return nil, fmt.Errorf("policy providers not configured. Make sure your policy group is referenced with file:// or https:// protocol")
	}

	var provider = uc.policyRegistry.DefaultProvider()
	if providerName != "" {
		provider = uc.policyRegistry.GetProvider(providerName)
	}

	if provider == nil {
		return nil, fmt.Errorf("failed to resolve provider: %s. Available providers: %s", providerName, uc.policyRegistry.GetProviderNames())
	}

	return provider, nil
}

// UnmarshalAndValidateRawContract Takes the raw contract + format and will unmarshal the contract and validate it
func UnmarshalAndValidateRawContract(raw []byte, format unmarshal.RawFormat) (*Contract, error) {
	// Try parsing as v2 Contract format first
	v2Contract := &schemav1.CraftingSchemaV2{}
	v2Err := unmarshal.FromRaw(raw, format, v2Contract, true)
	if v2Err == nil {
		// Custom Validations
		if err := v2Contract.ValidateUniqueMaterialName(); err != nil {
			return nil, NewErrValidation(fmt.Errorf("unique material name validation failed: %w", err))
		}
		if err := v2Contract.ValidatePolicyAttachments(); err != nil {
			return nil, NewErrValidation(fmt.Errorf("policy attachment validation failed: %w", err))
		}
		// Convert to v1 for backward compatibility with old CLIs
		v1Schema := v2Contract.ToV1()
		return &Contract{Raw: raw, Format: format, Schema: v1Schema, Schemav2: v2Contract}, nil
	}

	// Fallback to v1 CraftingSchema format
	// DEPRECATED: v1 schema is deprecated, use v2 Contract format instead
	v1Contract := &schemav1.CraftingSchema{}
	v1Err := unmarshal.FromRaw(raw, format, v1Contract, true)
	if v1Err == nil {
		// Custom Validations
		if err := v1Contract.ValidateUniqueMaterialName(); err != nil {
			return nil, NewErrValidation(fmt.Errorf("unique material name validation failed: %w", err))
		}
		if err := v1Contract.ValidatePolicyAttachments(); err != nil {
			return nil, NewErrValidation(fmt.Errorf("policy attachment validation failed: %w", err))
		}
		return &Contract{Raw: raw, Format: format, Schema: v1Contract}, nil
	}

	// Both parsing attempts failed
	// Best effort: provide errors for both schemas
	return nil, NewErrValidation(fmt.Errorf("contract validation failed:\n  v2 Contract format error: %w\n  v1 CraftingSchema format error: %w", v2Err, v1Err))
}

// Will try to figure out the format of the raw contract and validate it
func identifyUnMarshalAndValidateRawContract(raw []byte) (*Contract, error) {
	format, err := unmarshal.IdentifyFormat(raw)
	if err != nil {
		return nil, fmt.Errorf("identify contract: %w", err)
	}

	return UnmarshalAndValidateRawContract(raw, format)
}

// SchemaToRawContract generates a default representation of a contract
func SchemaToRawContract(contract *schemav1.CraftingSchema) (*Contract, error) {
	marshaler := protojson.MarshalOptions{Indent: "  "}
	r, err := marshaler.Marshal(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal contract: %w", err)
	}

	return &Contract{Raw: r, Format: unmarshal.RawFormatJSON, Schema: contract}, nil
}

// ContractScope represents a polymorphic relationship between a contract and a project or organization
type ContractScope string

const (
	ContractScopeProject ContractScope = "project"
	ContractScopeOrg     ContractScope = "org"
)

// Values implement https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (ContractScope) Values() (values []string) {
	values = append(values,
		string(ContractScopeProject),
		string(ContractScopeOrg),
	)

	return
}
