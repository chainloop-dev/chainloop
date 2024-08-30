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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cuelang.org/go/cue/cuecontext"
	"github.com/bufbuild/protovalidate-go"
	"github.com/bufbuild/protoyaml-go"
	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/policies"
	loader "github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"
)

type WorkflowContract struct {
	ID             uuid.UUID
	Name           string
	Description    string
	LatestRevision int
	CreatedAt      *time.Time
	// WorkflowNames is the list of workflows associated with this contract
	WorkflowNames []string
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
	Format ContractRawFormat
	// marhalled proto contract
	Schema *schemav1.CraftingSchema
}

type ContractRawFormat string

const (
	ContractRawFormatJSON ContractRawFormat = "json"
	ContractRawFormatYAML ContractRawFormat = "yaml"
	ContractRawFormatCUE  ContractRawFormat = "cue"
)

type WorkflowContractWithVersion struct {
	Contract *WorkflowContract
	Version  *WorkflowContractVersion
}

type WorkflowContractRepo interface {
	Create(ctx context.Context, opts *ContractCreateOpts) (*WorkflowContract, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*WorkflowContract, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*WorkflowContract, error)
	FindByNameInOrg(ctx context.Context, orgID uuid.UUID, name string) (*WorkflowContract, error)
	Describe(ctx context.Context, orgID, contractID uuid.UUID, revision int) (*WorkflowContractWithVersion, error)
	FindVersionByID(ctx context.Context, versionID uuid.UUID) (*WorkflowContractWithVersion, error)
	Update(ctx context.Context, orgID uuid.UUID, name string, opts *ContractUpdateOpts) (*WorkflowContractWithVersion, error)
	SoftDelete(ctx context.Context, contractID uuid.UUID) error
}

type ContractCreateOpts struct {
	Name        string
	OrgID       uuid.UUID
	Description *string
	// raw representation of the contract in whatever original format it was (json, yaml, ...)
	Contract *Contract
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
}

func NewWorkflowContractUseCase(repo WorkflowContractRepo, policyRegistry *policies.Registry, logger log.Logger) *WorkflowContractUseCase {
	return &WorkflowContractUseCase{repo: repo, policyRegistry: policyRegistry, logger: log.NewHelper(logger)}
}

func (uc *WorkflowContractUseCase) List(ctx context.Context, orgID string) ([]*WorkflowContract, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.List(ctx, orgUUID)
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

type WorkflowContractCreateOpts struct {
	OrgID, Name string
	RawSchema   []byte
	Description *string
	// Make sure that the name is unique in the organization
	AddUniquePrefix bool
}

// EmptyDefaultContract is the default contract that will be created if no contract is provided
var EmptyDefaultContract = &Contract{
	Raw: []byte("schemaVersion: v1"), Format: ContractRawFormatYAML,
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

	// Create an empty contract by default
	contract := EmptyDefaultContract

	// or load it if provided
	if len(opts.RawSchema) > 0 {
		c, err := identifyUnMarshalAndValidateRawContract(opts.RawSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to load contract: %w", err)
		}

		contract = c
	}

	// Create a workflow with a unique name if needed
	args := &ContractCreateOpts{
		OrgID: orgUUID, Name: opts.Name, Description: opts.Description,
		Contract: contract,
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

	args := &ContractUpdateOpts{Description: opts.Description, Contract: contract}
	c, err := uc.repo.Update(ctx, orgUUID, name, args)
	if err != nil {
		return nil, fmt.Errorf("failed to update contract: %w", err)
	} else if c == nil {
		return nil, NewErrNotFound("contract")
	}

	return c, nil
}

func (uc *WorkflowContractUseCase) ValidateContractPolicies(rawSchema []byte, token string) error {
	// Validate that externally provided policies exist
	c, err := identifyUnMarshalAndValidateRawContract(rawSchema)
	if err != nil {
		return NewErrValidation(err)
	}
	for _, att := range c.Schema.GetPolicies().GetAttestation() {
		_, err := uc.findPolicy(att, token)
		if err != nil {
			return NewErrValidation(err)
		}
	}
	for _, att := range c.Schema.GetPolicies().GetMaterials() {
		_, err := uc.findPolicy(att, token)
		if err != nil {
			return NewErrValidation(err)
		}
	}
	return nil
}

func (uc *WorkflowContractUseCase) findPolicy(att *schemav1.PolicyAttachment, token string) (*schemav1.Policy, error) {
	if att.GetEmbedded() != nil {
		return att.GetEmbedded(), nil
	}

	// if it should come from a provider, check that it's available
	// [chainloop://][provider/]name
	if loader.IsProviderScheme(att.GetRef()) {
		provider, name := loader.ProviderParts(att.GetRef())
		remotePolicy, err := uc.GetPolicy(provider, name, token)
		if err != nil {
			return nil, fmt.Errorf("failed to get policy '%s': %w", name, err)
		}
		return remotePolicy.Policy, nil
	}

	// Otherwise, don't return an error, as it might consist of a local policy, not available in this context
	return nil, nil
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

	if len(contract.WorkflowNames) > 0 {
		return NewErrValidation(errors.New("there are associated workflows with this contract, delete them first"))
	}

	// Check that the workflow to delete belongs to the provided organization
	return uc.repo.SoftDelete(ctx, contractUUID)
}

type RemotePolicy struct {
	ProviderRef *policies.PolicyReference
	Policy      *schemav1.Policy
}

// GetPolicy retrieves a policy from a policy provider
func (uc *WorkflowContractUseCase) GetPolicy(providerName, policyName, token string) (*RemotePolicy, error) {
	if len(uc.policyRegistry.GetProviderNames()) == 0 {
		return nil, fmt.Errorf("policy providers not configured. Make sure your policy is referenced with file:// or https:// protocol")
	}

	var provider = uc.policyRegistry.DefaultProvider()
	if providerName != "" {
		provider = uc.policyRegistry.GetProvider(providerName)
	}

	if provider == nil {
		return nil, fmt.Errorf("failed to resolve provider: %s. Available providers: %s", providerName, uc.policyRegistry.GetProviderNames())
	}

	policy, ref, err := provider.Resolve(policyName, token)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy: %w. Available providers: %s", err, uc.policyRegistry.GetProviderNames())
	}

	return &RemotePolicy{Policy: policy, ProviderRef: ref}, nil
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (ContractRawFormat) Values() (kinds []string) {
	for _, s := range []ContractRawFormat{ContractRawFormatJSON, ContractRawFormatYAML, ContractRawFormatCUE} {
		kinds = append(kinds, string(s))
	}
	return
}

// Take the raw contract + format and will unmarshal the contract and validate it
func UnmarshalAndValidateRawContract(raw []byte, format ContractRawFormat) (*Contract, error) {
	if format == "" {
		return nil, errors.New("format not provided")
	}

	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("could not create validator: %w", err)
	}

	contract := &schemav1.CraftingSchema{}

	switch format {
	case ContractRawFormatJSON:
		if err := protojson.Unmarshal(raw, contract); err != nil {
			return nil, NewErrValidation(err)
		}
	case ContractRawFormatCUE:
		ctx := cuecontext.New()
		v := ctx.CompileBytes(raw)
		jsonRawData, err := v.MarshalJSON()
		if err != nil {
			return nil, NewErrValidation(err)
		}

		if err := protojson.Unmarshal(jsonRawData, contract); err != nil {
			return nil, NewErrValidation(err)
		}
	case ContractRawFormatYAML:
		// protoyaml allows validating the contract while unmarshalling
		yamlOpts := protoyaml.UnmarshalOptions{Validator: validator}
		if err := yamlOpts.Unmarshal(raw, contract); err != nil {
			return nil, NewErrValidation(err)
		}
	}

	// Additional proto validations
	if err := validator.Validate(contract); err != nil {
		return nil, NewErrValidation(err)
	}

	// Custom Validations
	if err := contract.ValidateUniqueMaterialName(); err != nil {
		return nil, NewErrValidation(err)
	}

	return &Contract{Raw: raw, Format: format, Schema: contract}, nil
}

// Will try to figure out the format of the raw contract and validate it
func identifyUnMarshalAndValidateRawContract(raw []byte) (*Contract, error) {
	format, err := identifyFormat(raw)
	if err != nil {
		return nil, fmt.Errorf("identify contract: %w", err)
	}

	return UnmarshalAndValidateRawContract(raw, format)
}

// It does a best effort to identify the format of the raw contract
// by going the unmashalling path in the following order: json, cue, yaml
// NOTE that we are just validating the format, not the content using regular marshalling
// not even proto marshalling, that comes later once we know the format
func identifyFormat(raw []byte) (ContractRawFormat, error) {
	// json marshalling first
	var sink any
	if err := json.Unmarshal(raw, &sink); err == nil {
		return ContractRawFormatJSON, nil
	}

	// cue marshalling next
	ctx := cuecontext.New()
	v := ctx.CompileBytes(raw)
	if _, err := v.MarshalJSON(); err == nil {
		return ContractRawFormatCUE, nil
	}

	// yaml marshalling last
	if err := yaml.Unmarshal(raw, &sink); err == nil {
		return ContractRawFormatYAML, nil
	}

	return "", errors.New("format not found")
}

// generate a default representation of a contract
func SchemaToRawContract(contract *schemav1.CraftingSchema) (*Contract, error) {
	marshaler := protojson.MarshalOptions{Indent: "  "}
	r, err := marshaler.Marshal(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal contract: %w", err)
	}

	return &Contract{Raw: r, Format: ContractRawFormatJSON, Schema: contract}, nil
}
