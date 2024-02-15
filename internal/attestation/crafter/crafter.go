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

package crafter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue/cuecontext"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"
)

// StateManager is an interface for managing the state of the crafting process
type StateManager interface {
	// Check if the state is already initialized
	Initialized(ctx context.Context, key string) (bool, error)
	// Write the state to the manager backend
	Write(ctx context.Context, key string, state *api.CraftingState) error
	// Read the state from the manager backend
	Read(ctx context.Context, key string, state *api.CraftingState) error
	// Reset/Delete the state
	Reset(ctx context.Context, key string) error
	// String returns a string representation of the state manager
	Info(ctx context.Context, key string) string
}

type Crafter struct {
	logger        *zerolog.Logger
	CraftingState *api.CraftingState
	Runner        supportedRunner
	workingDir    string
	stateManager  StateManager
	// Authn is used to authenticate with the OCI registry
	ociRegistryAuth authn.Keychain
}

var ErrAttestationStateNotLoaded = errors.New("crafting state not loaded")

type NewOpt func(c *Crafter) error

func WithLogger(l *zerolog.Logger) NewOpt {
	return func(c *Crafter) error {
		c.logger = l
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

// Create a completely new crafter
func NewCrafter(stateManager StateManager, opts ...NewOpt) (*Crafter, error) {
	noopLogger := zerolog.Nop()

	cw, _ := os.Getwd()
	c := &Crafter{
		logger:       &noopLogger,
		workingDir:   cw,
		stateManager: stateManager,
		// By default we authenticate with the current user's keychain (i.e ~/.docker/config.json)
		ociRegistryAuth: authn.DefaultKeychain,
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
	// do not record, upload or push attestation
	DryRun bool
	// Identifier of the attestation state
	AttestationID string
}

// Initialize the crafter with a remote or local schema
func (c *Crafter) Init(ctx context.Context, opts *InitOpts) error {
	if opts.SchemaV1 == nil {
		return errors.New("schema is nil")
	} else if opts.WfInfo == nil {
		return errors.New("workflow metadata is nil")
	}

	// Check that the initialization is happening in the right environment
	runnerType := opts.SchemaV1.Runner.GetType()
	runnerContext := NewRunner(runnerType)
	if !opts.DryRun && !runnerContext.CheckEnv() {
		return fmt.Errorf("%w, expected %s", ErrRunnerContextNotFound, runnerType)
	}

	return c.initCraftingStateFile(ctx, opts.AttestationID, opts.SchemaV1, opts.WfInfo, opts.DryRun, runnerType, runnerContext.RunURI())
}

func (c *Crafter) AlreadyInitialized(ctx context.Context, stateID string) (bool, error) {
	return c.stateManager.Initialized(ctx, stateID)
}

// Extract raw data in JSON format from different sources, i.e cue or yaml files
func loadJSONBytes(rawData []byte, extension string) ([]byte, error) {
	var jsonRawData []byte
	var err error

	switch extension {
	case ".yaml", ".yml":
		jsonRawData, err = yaml.YAMLToJSON(rawData)
		if err != nil {
			return nil, err
		}
	case ".cue":
		ctx := cuecontext.New()
		v := ctx.CompileBytes(rawData)
		jsonRawData, err = v.MarshalJSON()
		if err != nil {
			return nil, err
		}
	case ".json":
		jsonRawData = rawData
	default:
		return nil, errors.New("unsupported file format")
	}

	return jsonRawData, nil
}

func LoadSchema(pathOrURI string) (*schemaapi.CraftingSchema, error) {
	// Extract json formatted data
	content, err := loadFileOrURL(pathOrURI)
	if err != nil {
		return nil, err
	}

	jsonSchemaRaw, err := loadJSONBytes(content, filepath.Ext(pathOrURI))
	if err != nil {
		return nil, err
	}

	schema := &schemaapi.CraftingSchema{}
	if err := protojson.Unmarshal(jsonSchemaRaw, schema); err != nil {
		return nil, err
	}

	// Proto validations
	if err := schema.ValidateAll(); err != nil {
		return nil, err
	}

	// Custom Validations
	if err := schema.ValidateUniqueMaterialName(); err != nil {
		return nil, err
	}

	return schema, nil
}

// Initialize the temporary file with the content of the schema
func (c *Crafter) initCraftingStateFile(
	ctx context.Context,
	attestationID string,
	schema *schemaapi.CraftingSchema,
	wf *api.WorkflowMetadata,
	dryRun bool,
	runnerType schemaapi.CraftingSchema_Runner_RunnerType,
	jobURL string,
) error {
	// Generate Crafting state
	state, err := initialCraftingState(c.workingDir, schema, wf, dryRun, runnerType, jobURL)
	if err != nil {
		return fmt.Errorf("initializing crafting state: %w", err)
	}

	if err := c.stateManager.Write(ctx, attestationID, state); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	c.logger.Debug().Str("state", c.stateManager.Info(ctx, attestationID)).Msg("created state file")

	return c.LoadCraftingState(ctx, attestationID)
}

// Reset removes the current crafting state
func (c *Crafter) Reset(ctx context.Context, stateID string) error {
	return c.stateManager.Reset(ctx, stateID)
}

func (c *Crafter) LoadCraftingState(ctx context.Context, attestationID string) error {
	c.logger.Debug().Str("state", c.stateManager.Info(ctx, attestationID)).Msg("loading state")

	c.CraftingState = &api.CraftingState{}

	if err := c.stateManager.Read(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to load crafting state: %w", err)
	}

	// Set runner too
	runnerType := c.CraftingState.GetInputSchema().GetRunner().GetType()
	if runnerType.String() == "" {
		return errors.New("runner type not set in the crafting state")
	}

	c.Runner = NewRunner(runnerType)
	c.logger.Debug().Str("state", c.stateManager.Info(ctx, attestationID)).Msg("loaded state")

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
	Message string
	Remotes []*CommitRemote
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

		c.Remotes = append(c.Remotes, &CommitRemote{
			Name: r.Config().Name,
			URL:  r.Config().URLs[0],
		})
	}

	return c, nil
}

func initialCraftingState(cwd string, schema *schemaapi.CraftingSchema, wf *api.WorkflowMetadata, dryRun bool, runnerType schemaapi.CraftingSchema_Runner_RunnerType, jobURL string) (*api.CraftingState, error) {
	// Get git commit hash
	headCommit, err := gracefulGitRepoHead(cwd)
	if err != nil {
		return nil, fmt.Errorf("getting git commit hash: %w", err)
	}

	var headCommitP *api.Commit
	if headCommit != nil {
		headCommitP = &api.Commit{
			Hash:        headCommit.Hash,
			AuthorEmail: headCommit.AuthorEmail,
			AuthorName:  headCommit.AuthorName,
			Date:        timestamppb.New(headCommit.Date),
			Message:     headCommit.Message,
		}

		for _, r := range headCommit.Remotes {
			headCommitP.Remotes = append(headCommitP.Remotes, &api.Commit_Remote{
				Name: r.Name,
				Url:  r.URL,
			})
		}
	}

	// Generate Crafting state
	return &api.CraftingState{
		InputSchema: schema,
		Attestation: &api.Attestation{
			InitializedAt: timestamppb.New(time.Now()),
			Workflow:      wf,
			RunnerType:    runnerType,
			RunnerUrl:     jobURL,
			Head:          headCommitP,
		},
		DryRun: dryRun,
	}, nil
}

// ResolveEnvVars will iterate on the env vars in the allow list and resolve them from the system context
// strict indicates if it should fail if any env variable can not be found
func (c *Crafter) ResolveEnvVars(ctx context.Context, attestationID string) error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	// Runner specific environment variables
	c.logger.Debug().Str("runnerType", c.Runner.String()).Msg("loading runner specific env variables")
	if !c.Runner.CheckEnv() {
		errorStr := fmt.Sprintf("couldn't detect the environment %q. Is the crafting process happening in the target env?", c.Runner.String())
		return fmt.Errorf("%s - %w", errorStr, ErrRunnerContextNotFound)
	}

	// Workflow run environment variables
	varNames := make([]string, len(c.Runner.ListEnvVars()))
	for index, envVarDef := range c.Runner.ListEnvVars() {
		varNames[index] = envVarDef.Name
	}
	c.logger.Debug().Str("runnerType", c.Runner.String()).Strs("variables", varNames).Msg("list of env variables to automatically extract")

	outputEnvVars, errors := c.Runner.ResolveEnvVars()
	if len(errors) > 0 {
		var combinedErrs string
		for _, err := range errors {
			combinedErrs += (*err).Error() + "\n"
		}
		return fmt.Errorf("error while resolving runner environment variables: %s", combinedErrs)
	}

	// User-defined environment vars
	if len(c.CraftingState.InputSchema.EnvAllowList) > 0 {
		c.logger.Debug().Strs("allowList", c.CraftingState.InputSchema.EnvAllowList).Msg("loading env variables")
	}
	for _, want := range c.CraftingState.InputSchema.EnvAllowList {
		val := os.Getenv(want)
		if val != "" {
			outputEnvVars[want] = val
		} else {
			return fmt.Errorf("required env variables not present %q", want)
		}
	}

	c.CraftingState.Attestation.EnvVars = outputEnvVars

	if err := c.stateManager.Write(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	return nil
}

// Inject material to attestation state
func (c *Crafter) AddMaterial(ctx context.Context, attestationID, key, value string, casBackend *casclient.CASBackend, runtimeAnnotations map[string]string) error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	// 1 - Check if the material to be added is in the schema
	var m *schemaapi.CraftingSchema_Material
	for _, d := range c.CraftingState.InputSchema.Materials {
		if d.Name == key {
			m = d
		}
	}

	if m == nil {
		return fmt.Errorf("material with id %q not found in the schema", key)
	}

	// 2 - Check that it has not been set yet and warn of override
	if _, found := c.CraftingState.Attestation.Materials[key]; found {
		c.logger.Info().Str("key", key).Str("value", value).Msg("material already set, overriding it")
	}

	// 3 - Craft resulting material
	mt, err := materials.Craft(context.Background(), m, value, casBackend, c.ociRegistryAuth, c.logger)
	if err != nil {
		return err
	}

	// 4 - Populate annotations from the ones provided at runtime
	// a) we do not allow overriding values that come from the contract
	// b) we do not allow adding annotations that are not defined in the contract
	for kr, vr := range runtimeAnnotations {
		// If the annotation is not defined in the material we fail
		if v, found := mt.Annotations[kr]; !found {
			return fmt.Errorf("annotation %q not found in material %q", kr, key)
		} else if v == "" {
			// Set it only if it's not set
			mt.Annotations[kr] = vr
		} else {
			// NOTE: we do not allow overriding values that come from the contract
			c.logger.Info().Str("key", key).Str("annotation", kr).Msg("annotation can't be changed, skipping")
		}
	}

	// Make sure all the annotation values are now set
	// This is in fact validated below but by manually checking we can provide a better error message
	for k, v := range mt.Annotations {
		var missingAnnotations []string
		if v == "" {
			missingAnnotations = append(missingAnnotations, k)
		}

		if len(missingAnnotations) > 0 {
			return fmt.Errorf("annotations %q required for material %q", missingAnnotations, key)
		}
	}

	if err := mt.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// 5 - Attach it to state
	if mt != nil {
		if c.CraftingState.Attestation.Materials == nil {
			c.CraftingState.Attestation.Materials = map[string]*api.Attestation_Material{key: mt}
		}
		c.CraftingState.Attestation.Materials[key] = mt
	}

	// 6 - Persist state
	if err := c.stateManager.Write(ctx, attestationID, c.CraftingState); err != nil {
		return fmt.Errorf("failed to persist crafting state: %w", err)
	}

	c.logger.Debug().Str("key", key).Msg("added to state")
	return nil
}

func (c *Crafter) ValidateAttestation() error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	return c.CraftingState.ValidateComplete()
}

func (c *Crafter) requireStateLoaded() error {
	if c.CraftingState == nil {
		return ErrAttestationStateNotLoaded
	}

	return nil
}

func loadFileOrURL(fileRef string) ([]byte, error) {
	parts := strings.SplitAfterN(fileRef, "://", 2)
	if len(parts) == 2 {
		scheme := parts[0]
		switch scheme {
		case "http://":
			fallthrough
		case "https://":
			// #nosec G107
			resp, err := http.Get(fileRef)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		default:
			return nil, errors.New("invalid file scheme")
		}
	}

	return os.ReadFile(filepath.Clean(fileRef))
}
