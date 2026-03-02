//
// Copyright 2024-2026 The Chainloop Authors.
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

package crafter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
	"github.com/chainloop-dev/chainloop/internal/prinfo"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-containerregistry/pkg/authn"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StateManager is an interface for managing the state of the crafting process
type StateManager interface {
	// Check if the state is already initialized
	Initialized(ctx context.Context, key string) (bool, error)
	// Write the state to the manager backend
	Write(ctx context.Context, key string, state *VersionedCraftingState) error
	// Read the state from the manager backend
	Read(ctx context.Context, key string, state *VersionedCraftingState) error
	// Reset/Delete the state
	Reset(ctx context.Context, key string) error
	// String returns a string representation of the state manager
	Info(ctx context.Context, key string) string
}

type Crafter struct {
	Logger        *zerolog.Logger
	AuthRawToken  string
	CraftingState *VersionedCraftingState
	Runner        SupportedRunner
	workingDir    string
	stateManager  StateManager
	// Authn is used to authenticate with the OCI registry
	ociRegistryAuth authn.Keychain
	validator       protovalidate.Validator

	// attestation client is used to load chainloop policies
	attClient v1.AttestationServiceClient

	// noStrictValidation skips strict schema validation
	noStrictValidation bool
}

type VersionedCraftingState struct {
	*api.CraftingState
	// This digest is used to verify the integrity of the state during updates
	UpdateCheckSum string
}

var ErrAttestationStateNotLoaded = errors.New("crafting state not loaded")

type NewOpt func(c *Crafter) error

func WithAuthRawToken(token string) NewOpt {
	return func(c *Crafter) error {
		c.AuthRawToken = token
		return nil
	}
}

func WithLogger(l *zerolog.Logger) NewOpt {
	return func(c *Crafter) error {
		c.Logger = l
		return nil
	}
}

func WithWorkingDirPath(path string) NewOpt {
	return func(c *Crafter) error {
		c.workingDir = path
		return nil
	}
}

func WithOCIAuth(server, username, password string) NewOpt {
	return func(c *Crafter) error {
		k, err := ociauth.NewCredentialsFromRegistry(server, username, password)
		if err != nil {
			return fmt.Errorf("failed to load OCI credentials: %w", err)
		}

		c.ociRegistryAuth = k
		return nil
	}
}

func WithNoStrictValidation(noStrictValidation bool) NewOpt {
	return func(c *Crafter) error {
		c.noStrictValidation = noStrictValidation
		return nil
	}
}

// Create a completely new crafter
func NewCrafter(stateManager StateManager, attClient v1.AttestationServiceClient, opts ...NewOpt) (*Crafter, error) {
	noopLogger := zerolog.Nop()

	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("creating proto validator: %w", err)
	}

	cw, _ := os.Getwd()
	c := &Crafter{
		Logger:       &noopLogger,
		workingDir:   cw,
		stateManager: stateManager,
		// By default we authenticate with the current user's keychain (i.e ~/.docker/config.json)
		ociRegistryAuth: authn.DefaultKeychain,
		validator:       validator,
		attClient:       attClient,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

type InitOpts struct {
	// Control plane workflow metadata
	WfInfo *api.WorkflowMetadata
	// already marshaled schema
	SchemaV1 *schemaapi.CraftingSchema
	// already marshaled schema
	SchemaV2 *schemaapi.CraftingSchemaV2
	// do not record, upload or push attestation
	DryRun bool
	// Identifier of the attestation state
	AttestationID string
	Runner        SupportedRunner
	// fail the attestation if policy evaluation fails
	BlockOnPolicyViolation bool
	// Signing options
	SigningOptions *SigningOpts
	// Authentication token
	Auth *api.Attestation_Auth
	// array of hostnames that are allowed to be used in the policies
	PoliciesAllowedHostnames []string
	// CAS backend information
	CASBackend *api.Attestation_CASBackend
	// UIDashboardURL is the base URL to build the attestation view link
	UIDashboardURL string
	// Logger for verification logging
	Logger *zerolog.Logger
}

type SigningOpts struct {
	// Timestamp Authority to use
	TimestampAuthorityURL string
	// Signing CA name
	SigningCAName string
}

// Init initializes the crafter with a remote or local schema
func (c *Crafter) Init(ctx context.Context, opts *InitOpts) error {
	if opts.SchemaV1 == nil && opts.SchemaV2 == nil {
		return errors.New("schema is nil")
	} else if opts.WfInfo == nil {
		return errors.New("workflow metadata is nil")
	}

	return c.initCraftingStateFile(ctx, opts)
}

func (c *Crafter) AlreadyInitialized(ctx context.Context, stateID string) (bool, error) {
	return c.stateManager.Initialized(ctx, stateID)
}

// Initialize the temporary file with the content of the schema
func (c *Crafter) initCraftingStateFile(ctx context.Context, opts *InitOpts) error {
	// Generate Crafting state
	state, err := initialCraftingState(c.workingDir, opts)
	if err != nil {
		return fmt.Errorf("initializing crafting state: %w", err)
	}

	// newState doesn't have a digest to check against
	newState := &VersionedCraftingState{CraftingState: state}
	if err := c.stateManager.Write(ctx, opts.AttestationID, newState); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	c.Logger.Debug().Str("state", c.stateManager.Info(ctx, opts.AttestationID)).Msg("created state file")

	return c.LoadCraftingState(ctx, opts.AttestationID)
}

// Reset removes the current crafting state
func (c *Crafter) Reset(ctx context.Context, stateID string) error {
	return c.stateManager.Reset(ctx, stateID)
}

func (c *Crafter) LoadCraftingState(ctx context.Context, attestationID string) error {
	c.Logger.Debug().Str("state", c.stateManager.Info(ctx, attestationID)).Msg("loading state")

	c.CraftingState = &VersionedCraftingState{CraftingState: &api.CraftingState{}}

	if err := c.stateManager.Read(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to load crafting state: %w", err)
	}

	// Set runner too
	runnerType := c.CraftingState.GetAttestation().GetRunnerType()
	if runnerType.String() == "" {
		return errors.New("runner type not set in the crafting state")
	}

	c.Runner = NewRunner(runnerType, c.AuthRawToken, c.Logger)
	c.Logger.Debug().Str("state", c.stateManager.Info(ctx, attestationID)).Msg("loaded state")

	return nil
}

type HeadCommit struct {
	// hash of the commit
	Hash string
	// When did the commit happen
	Date time.Time
	// Author of the commit
	AuthorEmail, AuthorName string
	// Commit Message
	Message   string
	Remotes   []*CommitRemote
	Signature string
	// Platform verification (if available)
	PlatformVerification *api.Commit_CommitVerification
}

type CommitRemote struct {
	Name, URL string
}

// This error is not exposed by go-git
var errBranchInvalidMerge = errors.New("branch config: invalid merge")

// Returns the current directory git commit hash if possible
// If we are not in a git repo it will return an empty string
func gracefulGitRepoHead(path string) (*HeadCommit, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		// walk up the directory tree until we find a git repo
		DetectDotGit: true,
	})

	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, nil
		}

		return nil, fmt.Errorf("opening repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding repo head: %w", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("finding head commit: %w", err)
	}

	c := &HeadCommit{
		Hash:        commit.Hash.String(),
		AuthorEmail: commit.Author.Email,
		AuthorName:  commit.Author.Name,
		Date:        commit.Author.When,
		Message:     commit.Message,
		Remotes:     make([]*CommitRemote, 0),
		Signature:   commit.PGPSignature,
	}

	remotes, err := repo.Remotes()
	if err != nil {
		// go-git does an additional validation that the branch is pushed upstream
		// we do not care about that use-case, so we ignore the error
		// we compare by error string because go-git does not expose the error type
		// and errors.Is require the same instance of the error
		if err.Error() == errBranchInvalidMerge.Error() {
			return c, nil
		}

		return nil, fmt.Errorf("getting remotes: %w", err)
	}

	for _, r := range remotes {
		if err := r.Config().Validate(); err != nil {
			continue
		}

		remoteURI, err := sanitizeRemoteURL(r.Config().URLs[0])
		if err != nil {
			return nil, fmt.Errorf("sanitizing remote url: %w", err)
		}

		c.Remotes = append(c.Remotes, &CommitRemote{
			Name: r.Config().Name,
			URL:  remoteURI,
		})
	}

	return c, nil
}

// Clear any basic auth credentials from the remote URL
func sanitizeRemoteURL(remoteURL string) (string, error) {
	uri, err := url.Parse(remoteURL)
	if err != nil {
		// check if it's a valid git@ url
		if strings.HasPrefix(remoteURL, "git@") {
			return remoteURL, nil
		}

		return "", fmt.Errorf("parsing remote url: %w", err)
	}

	// clear basic auth credentials
	uri.User = nil
	return uri.String(), nil
}

func initialCraftingState(cwd string, opts *InitOpts) (*api.CraftingState, error) {
	if opts.WfInfo == nil || opts.Runner == nil || (opts.SchemaV1 == nil && opts.SchemaV2 == nil) {
		return nil, errors.New("required init options not provided")
	}
	// Get git commit hash
	headCommit, err := gracefulGitRepoHead(cwd)
	if err != nil {
		return nil, fmt.Errorf("getting git commit hash: %w", err)
	}

	var headCommitP *api.Commit
	if headCommit != nil {
		// Attempt platform verification
		if opts.Runner != nil {
			headCommit.PlatformVerification = verifyCommitWithPlatform(headCommit, opts.Runner)
		}

		headCommitP = &api.Commit{
			Hash:                 headCommit.Hash,
			AuthorEmail:          headCommit.AuthorEmail,
			AuthorName:           headCommit.AuthorName,
			Date:                 timestamppb.New(headCommit.Date),
			Message:              headCommit.Message,
			Signature:            headCommit.Signature,
			PlatformVerification: headCommit.PlatformVerification,
		}

		for _, r := range headCommit.Remotes {
			headCommitP.Remotes = append(headCommitP.Remotes, &api.Commit_Remote{
				Name: r.Name,
				Url:  r.URL,
			})
		}
	}

	var tsURL, caName string
	if opts.SigningOptions != nil {
		tsURL = opts.SigningOptions.TimestampAuthorityURL
		caName = opts.SigningOptions.SigningCAName
	}

	// Generate Crafting state
	craftingState := &api.CraftingState{
		Attestation: &api.Attestation{
			InitializedAt:          timestamppb.New(time.Now()),
			Workflow:               opts.WfInfo,
			RunnerType:             opts.Runner.ID(),
			RunnerUrl:              opts.Runner.RunURI(),
			Head:                   headCommitP,
			BlockOnPolicyViolation: opts.BlockOnPolicyViolation,
			SigningOptions: &api.Attestation_SigningOptions{
				TimestampAuthorityUrl: tsURL,
				SigningCa:             caName,
			},
			RunnerEnvironment: &api.RunnerEnvironment{
				WorkflowFilePath: opts.Runner.WorkflowFilePath(),
				Environment:      opts.Runner.Environment().String(),
				Authenticated:    opts.Runner.IsAuthenticated(),
				Type:             opts.Runner.ID(),
				Url:              opts.Runner.RunURI(),
			},
			Auth:                     opts.Auth,
			PoliciesAllowedHostnames: opts.PoliciesAllowedHostnames,
			CasBackend:               opts.CASBackend,
		},
		DryRun:         opts.DryRun,
		UiDashboardUrl: opts.UIDashboardURL,
	}

	// Set the appropriate schema
	if opts.SchemaV2 != nil {
		craftingState.Schema = &api.CraftingState_SchemaV2{
			SchemaV2: opts.SchemaV2,
		}
	} else {
		craftingState.Schema = &api.CraftingState_InputSchema{
			InputSchema: opts.SchemaV1,
		}
	}

	return craftingState, nil
}

// ResolveEnvVars will iterate on the env vars in the allow list and resolve them from the system context
// strict indicates if it should fail if any env variable can not be found
func (c *Crafter) ResolveEnvVars(ctx context.Context, attestationID string) error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	// Runner specific environment variables
	c.Logger.Debug().Str("runnerType", c.Runner.ID().String()).Msg("loading runner specific env variables")
	if !c.Runner.CheckEnv() {
		errorStr := fmt.Sprintf("couldn't detect the environment %q. Is the crafting process happening in the target env?", c.Runner.ID().String())
		return fmt.Errorf("%s - %w", errorStr, ErrRunnerContextNotFound)
	}

	// Workflow run environment variables
	varNames := make([]string, len(c.Runner.ListEnvVars()))
	for index, envVarDef := range c.Runner.ListEnvVars() {
		varNames[index] = envVarDef.Name
	}
	c.Logger.Debug().Str("runnerType", c.Runner.ID().String()).Strs("variables", varNames).Msg("list of env variables to automatically extract")

	outputEnvVars, errors := c.Runner.ResolveEnvVars()
	if len(errors) > 0 {
		var combinedErrs string
		for _, err := range errors {
			combinedErrs += (*err).Error() + "\n"
		}
		return fmt.Errorf("error while resolving runner environment variables: %s", combinedErrs)
	}

	// User-defined environment vars
	envAllowList := c.CraftingState.GetEnvAllowList()
	if len(envAllowList) > 0 {
		c.Logger.Debug().Strs("allowList", envAllowList).Msg("loading env variables")
	}
	for _, want := range envAllowList {
		val := os.Getenv(want)
		if val != "" {
			outputEnvVars[want] = val
		} else {
			return fmt.Errorf("required env variables not present %q", want)
		}
	}

	// Resolve runner information
	c.resolveRunnerInfo()

	c.CraftingState.Attestation.EnvVars = outputEnvVars

	if err := c.stateManager.Write(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	return nil
}

// AutoCollectPRMetadata automatically collects PR/MR metadata if running in a PR/MR context
func (c *Crafter) AutoCollectPRMetadata(ctx context.Context, attestationID string, runner SupportedRunner, casBackend *casclient.CASBackend) error {
	if err := c.requireStateLoaded(); err != nil {
		return fmt.Errorf("crafting state not loaded before inspecting PR/MR metadata: %w", err)
	}

	if err := c.LoadCraftingState(ctx, attestationID); err != nil {
		c.Logger.Warn().Err(err).Msg("failed to reload crafting state")
	}

	// Detect if we're in a PR/MR context
	isPR, metadata, err := DetectPRContext(runner)
	if err != nil {
		return fmt.Errorf("failed to detect PR/MR context: %w", err)
	}

	// If not in PR/MR context, nothing to do
	if !isPR {
		c.Logger.Debug().Msg("not in PR/MR context, skipping metadata collection")
		return nil
	}

	c.Logger.Info().Str("platform", metadata.Platform).Str("number", metadata.Number).Msg("detected PR/MR context")

	// Create the material
	evidenceData := prinfo.NewEvidence(prinfo.Data{
		Platform:     metadata.Platform,
		Type:         metadata.Type,
		Number:       metadata.Number,
		Title:        metadata.Title,
		Description:  metadata.Description,
		SourceBranch: metadata.SourceBranch,
		TargetBranch: metadata.TargetBranch,
		URL:          metadata.URL,
		Author:       metadata.Author,
	})

	// Marshal to JSON
	jsonData, err := json.Marshal(evidenceData)
	if err != nil {
		return fmt.Errorf("failed to marshal PR/MR metadata: %w", err)
	}

	// Create a temporary file for the metadata
	materialName := fmt.Sprintf("pr-metadata-%s", metadata.Number)
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s.json", materialName))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the JSON data to the temp file
	if _, err := tmpFile.Write(jsonData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write metadata to temp file: %w", err)
	}
	tmpFile.Close()

	// Add the material using the crafter with explicit CHAINLOOP_PR_INFO type
	if _, err := c.AddMaterialContractFree(ctx, attestationID, schemaapi.CraftingSchema_Material_CHAINLOOP_PR_INFO.String(), materialName, tmpFile.Name(), casBackend, nil); err != nil {
		return fmt.Errorf("failed to add PR/MR metadata material: %w", err)
	}

	c.Logger.Info().Msg("successfully collected and attested PR/MR metadata")
	return nil
}

func (c *Crafter) resolveRunnerInfo() {
	c.CraftingState.Attestation.RunnerEnvironment = &api.RunnerEnvironment{
		Environment:      c.Runner.Environment().String(),
		Authenticated:    c.Runner.IsAuthenticated(),
		WorkflowFilePath: c.Runner.WorkflowFilePath(),
		Type:             c.Runner.ID(),
		Url:              c.Runner.RunURI(),
	}
}

// AddMaterialContractFree adds a material to the crafting state without checking the contract schema.
// This is useful for adding materials that are not defined in the schema.
// The name of the material is automatically calculated to conform the API contract if not provided.
func (c *Crafter) AddMaterialContractFree(ctx context.Context, attestationID, kind, name, value string, casBackend *casclient.CASBackend, runtimeAnnotations map[string]string) (*api.Attestation_Material, error) {
	if err := c.requireStateLoaded(); err != nil {
		return nil, fmt.Errorf("adding materials outisde the contract: %w", err)
	}

	// 1 - Try to parse incoming type to a known kind
	m := schemaapi.CraftingSchema_Material{Optional: true}
	if val, found := schemaapi.CraftingSchema_Material_MaterialType_value[kind]; found {
		m.Type = schemaapi.CraftingSchema_Material_MaterialType(val)
	} else {
		return nil, fmt.Errorf("%q kind not found. Available options are %q", kind, schemaapi.ListAvailableMaterialKind())
	}

	// 2 - Set the name of the material if provided
	m.Name = name
	if m.Name == "" {
		// 2.1 - Generate a random name for the material since it was not provided
		m.Name = fmt.Sprintf("material-%d", time.Now().UnixNano())
	}

	// 3 - Craft resulting material
	return c.addMaterial(ctx, &m, attestationID, value, casBackend, runtimeAnnotations)
}

// AddMaterialFromContract adds a material to the crafting state checking the incoming materials is
// in the schema and has not been set yet
func (c *Crafter) AddMaterialFromContract(ctx context.Context, attestationID, key, value string, casBackend *casclient.CASBackend, runtimeAnnotations map[string]string) (*api.Attestation_Material, error) {
	if err := c.requireStateLoaded(); err != nil {
		return nil, fmt.Errorf("adding materials outisde from contract: %w", err)
	}

	// 1 - Check if the material to be added is in the schema
	var m *schemaapi.CraftingSchema_Material
	for _, d := range c.CraftingState.GetMaterials() {
		if d.Name == key {
			m = d
		}
	}

	if m == nil {
		return nil, fmt.Errorf("material with id %q not found in the schema", key)
	}

	// 2 - Check that it has not been set yet and warn of override
	if _, found := c.CraftingState.Attestation.Materials[key]; found {
		c.Logger.Info().Str("key", key).Str("value", value).Msg("material already set, overriding it")
	}

	// 3 - Craft resulting material
	return c.addMaterial(ctx, m, attestationID, value, casBackend, runtimeAnnotations)
}

// IsMaterialInContract checks if the material is in the contract schema
func (c *Crafter) IsMaterialInContract(key string) bool {
	if err := c.requireStateLoaded(); err != nil {
		return false
	}

	for _, d := range c.CraftingState.GetMaterials() {
		if d.Name == key {
			return true
		}
	}

	return false
}

// AddMaterialContactFreeWithAutoDetectedKind adds a material to the crafting state checking the incoming material matches any of the
// supported types in validation order. If the material is not found it will return an error.
func (c *Crafter) AddMaterialContactFreeWithAutoDetectedKind(ctx context.Context, attestationID, name, value string, casBackend *casclient.CASBackend, runtimeAnnotations map[string]string) (*api.Attestation_Material, error) {
	var err error
	for _, kind := range schemaapi.CraftingMaterialInValidationOrder {
		m, err := c.AddMaterialContractFree(ctx, attestationID, kind.String(), name, value, casBackend, runtimeAnnotations)
		if err == nil {
			// Successfully added material, return the kind
			return m, nil
		}

		c.Logger.Debug().Err(err).Str("kind", kind.String()).Msg("failed to add material")

		// Handle base error for upload and craft errors except the opening file error
		// TODO: have an error to detect validation error instead
		var policyError *policies.PolicyError
		if errors.Is(err, materials.ErrBaseUploadAndCraft) || errors.As(err, &policyError) {
			return nil, err
		}

		// This is a final error, we detected the kind
		if v1.IsAttestationStateErrorConflict(err) {
			return nil, err
		}
	}

	// Return an error if no material could be added
	return nil, fmt.Errorf("failed to auto-discover material kind: %w", err)
}

// addMaterials adds the incoming material m to the crafting state
func (c *Crafter) addMaterial(ctx context.Context, m *schemaapi.CraftingSchema_Material, attestationID, value string, casBackend *casclient.CASBackend, runtimeAnnotations map[string]string) (*api.Attestation_Material, error) {
	// 3- Craft resulting material
	mt, err := materials.Craft(context.Background(), m, value, casBackend, c.ociRegistryAuth, c.Logger, &materials.CraftingOpts{
		NoStrictValidation: c.noStrictValidation,
	})
	if err != nil {
		return nil, err
	}

	// 4 - Populate annotations from the ones provided at runtime
	// a) we do not allow overriding values that come from the contract
	// b) we allow adding annotations that are not defined in the contract
	for kr, vr := range runtimeAnnotations {
		if mt.Annotations == nil {
			mt.Annotations = make(map[string]string)
		}

		// NOTE: we do not allow overriding values that come from the contract
		if existingVal, existsInContract := mt.Annotations[kr]; existsInContract && existingVal != "" {
			c.Logger.Info().Str("key", vr).Str("annotation", kr).Msg("annotation value is set in the contract, can not be overridden, skipping")
			continue
		}

		mt.Annotations[kr] = vr
	}

	// Make sure all the annotation values are now set
	// This is in fact validated below but by manually checking we can provide a better error message
	for k, v := range mt.Annotations {
		var missingAnnotations []string
		if v == "" {
			missingAnnotations = append(missingAnnotations, k)
		}

		if len(missingAnnotations) > 0 {
			return nil, fmt.Errorf("annotations %q required for material %q", missingAnnotations, m.Name)
		}
	}

	if err := c.validator.Validate(mt); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Remove existing policy evaluations for this material
	// since the value might have changed
	c.CraftingState.Attestation.PolicyEvaluations = slices.DeleteFunc(c.CraftingState.Attestation.PolicyEvaluations, func(i *api.PolicyEvaluation) bool {
		return i.MaterialName == m.Name
	})

	pgv := policies.NewPolicyGroupVerifier(
		c.CraftingState.GetPolicyGroups(),
		c.CraftingState.GetPolicies(),
		c.attClient,
		c.Logger,
		policies.WithAllowedHostnames(c.CraftingState.Attestation.PoliciesAllowedHostnames...),
		policies.WithDefaultGate(c.CraftingState.Attestation.GetBlockOnPolicyViolation()),
	)
	policyGroupResults, err := pgv.VerifyMaterial(ctx, mt, value)
	if err != nil {
		return nil, fmt.Errorf("error applying policy groups to material: %w", err)
	}
	c.CraftingState.Attestation.PolicyEvaluations = append(c.CraftingState.Attestation.PolicyEvaluations, policyGroupResults...)

	// log group policy violations
	policies.LogPolicyEvaluations(policyGroupResults, c.Logger)

	// Validate policies
	pv := policies.NewPolicyVerifier(
		c.CraftingState.GetPolicies(),
		c.attClient,
		c.Logger,
		policies.WithAllowedHostnames(c.CraftingState.Attestation.PoliciesAllowedHostnames...),
		policies.WithDefaultGate(c.CraftingState.Attestation.GetBlockOnPolicyViolation()),
	)
	policyResults, err := pv.VerifyMaterial(ctx, mt, value)
	if err != nil {
		return nil, fmt.Errorf("error applying policies to material: %w", err)
	}

	// store policy results
	c.CraftingState.Attestation.PolicyEvaluations = append(c.CraftingState.Attestation.PolicyEvaluations, policyResults...)

	// log policy violations
	policies.LogPolicyEvaluations(policyResults, c.Logger)

	// 5 - Attach it to state
	if c.CraftingState.Attestation.Materials == nil {
		c.CraftingState.Attestation.Materials = map[string]*api.Attestation_Material{m.Name: mt}
	}
	c.CraftingState.Attestation.Materials[m.Name] = mt

	// 6 - Persist state
	if err := c.stateManager.Write(ctx, attestationID, c.CraftingState); err != nil {
		return nil, fmt.Errorf("failed to persist crafting state: %w", err)
	}

	c.Logger.Debug().Str("key", m.Name).Msg("added to state")
	return mt, nil
}

// EvaluateAttestationPolicies evaluates the attestation-level policies and stores them in the attestation state.
// The phase parameter controls which policies are evaluated based on their attestation_phases spec field.
func (c *Crafter) EvaluateAttestationPolicies(ctx context.Context, attestationID string, statement *intoto.Statement, phase policies.EvalPhase) error {
	// evaluate attestation-level policies
	pv := policies.NewPolicyVerifier(c.CraftingState.GetPolicies(), c.attClient, c.Logger,
		policies.WithAllowedHostnames(c.CraftingState.Attestation.PoliciesAllowedHostnames...),
		policies.WithDefaultGate(c.CraftingState.Attestation.GetBlockOnPolicyViolation()),
		policies.WithEvalPhase(phase),
	)
	policyEvaluations, err := pv.VerifyStatement(ctx, statement)
	if err != nil {
		return fmt.Errorf("evaluating policies in statement: %w", err)
	}

	pgv := policies.NewPolicyGroupVerifier(c.CraftingState.GetPolicyGroups(), c.CraftingState.GetPolicies(), c.attClient, c.Logger,
		policies.WithAllowedHostnames(c.CraftingState.Attestation.PoliciesAllowedHostnames...),
		policies.WithDefaultGate(c.CraftingState.Attestation.GetBlockOnPolicyViolation()),
		policies.WithEvalPhase(phase),
	)
	policyGroupResults, err := pgv.VerifyStatement(ctx, statement)
	if err != nil {
		return fmt.Errorf("evaluating policy groups in statement: %w", err)
	}

	// Eliminate duplicates by checking if they have been already evaluated
	// by comparing the policy reference and its arguments
	policyEvaluations = append(policyEvaluations, policyGroupResults...)
	var filteredPolicyEvaluations []*api.PolicyEvaluation
	for _, ev := range policyEvaluations {
		var duplicated bool
		for _, existing := range filteredPolicyEvaluations {
			if proto.Equal(existing.PolicyReference, ev.PolicyReference) && reflect.DeepEqual(existing.With, ev.With) {
				duplicated = true
				break
			}
		}

		if !duplicated {
			filteredPolicyEvaluations = append(filteredPolicyEvaluations, ev)
		}
	}

	policyEvaluations = filteredPolicyEvaluations

	// Preserve existing evaluations that were not re-evaluated in this phase:
	// - Material-level evaluations are always kept
	// - Attestation-level evaluations are kept if they weren't re-evaluated (e.g., from a different phase)
	for _, ev := range c.CraftingState.Attestation.PolicyEvaluations {
		if ev.MaterialName != "" {
			policyEvaluations = append(policyEvaluations, ev)
			continue
		}

		// Check if this attestation-level evaluation was re-evaluated in the current phase
		var reEvaluated bool
		for _, newEv := range policyEvaluations {
			if proto.Equal(newEv.PolicyReference, ev.PolicyReference) && reflect.DeepEqual(newEv.With, ev.With) {
				reEvaluated = true
				break
			}
		}

		if !reEvaluated {
			policyEvaluations = append(policyEvaluations, ev)
		}
	}

	c.CraftingState.Attestation.PolicyEvaluations = policyEvaluations

	if err := c.stateManager.Write(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	return nil
}

func (c *Crafter) ValidateAttestation() error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	return c.CraftingState.ValidateComplete(c.CraftingState.GetDryRun())
}

func (c *Crafter) requireStateLoaded() error {
	if c.CraftingState == nil {
		return ErrAttestationStateNotLoaded
	}

	return nil
}

// verifyCommitWithPlatform attempts to verify commit signature using platform APIs
// Returns nil if verification is not available or not applicable
func verifyCommitWithPlatform(commit *HeadCommit, runner SupportedRunner) *api.Commit_CommitVerification {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call runner's verification method directly
	verification := runner.VerifyCommitSignature(ctx, commit.Hash)
	if verification == nil {
		return nil
	}

	// Convert from commitverification type to protobuf type
	return convertCommitVerification(verification)
}

// convertCommitVerification converts from commitverification.CommitVerification to protobuf type
func convertCommitVerification(v *commitverification.CommitVerification) *api.Commit_CommitVerification {
	if v == nil {
		return nil
	}

	// Convert status enum
	var status api.Commit_CommitVerification_VerificationStatus
	switch v.Status {
	case commitverification.VerificationStatusVerified:
		status = api.Commit_CommitVerification_verified
	case commitverification.VerificationStatusUnverified:
		status = api.Commit_CommitVerification_unverified
	case commitverification.VerificationStatusUnavailable:
		status = api.Commit_CommitVerification_unavailable
	case commitverification.VerificationStatusNotApplicable:
		status = api.Commit_CommitVerification_not_applicable
	default:
		status = api.Commit_CommitVerification_unspecified
	}

	return &api.Commit_CommitVerification{
		Attempted:          v.Attempted,
		Status:             status,
		Reason:             v.Reason,
		Platform:           v.Platform,
		KeyId:              v.KeyID,
		SignatureAlgorithm: v.SignatureAlgorithm,
	}
}
