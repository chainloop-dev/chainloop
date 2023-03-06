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
	api "github.com/chainloop-dev/bedrock/app/cli/api/attestation/v1"
	schemaapi "github.com/chainloop-dev/bedrock/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/bedrock/internal/attestation/crafter/materials"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"
)

type Crafter struct {
	logger        *zerolog.Logger
	statePath     string
	CraftingState *api.CraftingState
	uploader      materials.Uploader
	Runner        supportedRunner
}

var ErrAttestationStateNotLoaded = errors.New("crafting state not loaded")

type NewOpt func(c *Crafter)

func WithUploader(uploader materials.Uploader) NewOpt {
	return func(c *Crafter) {
		c.uploader = uploader
	}
}

// where to store the attestation state file
func WithStatePath(path string) NewOpt {
	return func(c *Crafter) {
		c.statePath = path
	}
}

func WithLogger(l *zerolog.Logger) NewOpt {
	return func(c *Crafter) {
		c.logger = l
	}
}

// Create a completely new crafter
func NewCrafter(opts ...NewOpt) *Crafter {
	noopLogger := zerolog.Nop()
	defaultStatePath := filepath.Join(os.TempDir(), "chainloop_attestation.tmp.json")

	c := &Crafter{
		logger:    &noopLogger,
		statePath: defaultStatePath,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type InitOpts struct {
	// Control plane workflow metadata
	WfInfo *api.WorkflowMetadata
	// already marshalled schema
	SchemaV1 *schemaapi.CraftingSchema
	// do not record, upload or push attestation
	DryRun bool
}

// Initialize the crafter with a remote or local schema
func (c *Crafter) Init(opts *InitOpts) error {
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

	return c.initCraftingStateFile(opts.SchemaV1, opts.WfInfo, opts.DryRun, runnerType, runnerContext.RunURI())
}

func (c *Crafter) AlreadyInitialized() bool {
	if file, err := os.Stat(c.statePath); err != nil {
		return false
	} else if file != nil {
		return true
	}

	return false
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
func (c *Crafter) initCraftingStateFile(schema *schemaapi.CraftingSchema, wf *api.WorkflowMetadata, dryRun bool, runnerType schemaapi.CraftingSchema_Runner_RunnerType, jobURL string) error {
	// Generate Crafting state
	state := initialCraftingState(schema, wf, dryRun, runnerType, jobURL)
	if err := persistCraftingState(state, c.statePath); err != nil {
		return err
	}

	c.logger.Debug().Str("path", c.statePath).Msg("created state file")

	return c.LoadCraftingState()
}

// Reset removes the current crafting state
func (c *Crafter) Reset() error {
	c.logger.Debug().Str("path", c.statePath).Msg("removing")
	return os.Remove(c.statePath)
}

func (c *Crafter) LoadCraftingState() error {
	file, err := os.Open(c.statePath)
	if err != nil {
		return err
	}
	defer file.Close()

	state := &api.CraftingState{}
	stateRaw, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := protojson.Unmarshal(stateRaw, state); err != nil {
		return err
	}

	c.CraftingState = state

	// Set runner too
	runnerType := state.GetInputSchema().GetRunner().GetType()
	if runnerType.String() == "" {
		return errors.New("runner type not set in the crafting state")
	}

	c.Runner = NewRunner(runnerType)

	c.logger.Debug().Str("path", c.statePath).Msg("loaded state file")
	return nil
}

func initialCraftingState(schema *schemaapi.CraftingSchema, wf *api.WorkflowMetadata, dryRun bool, runnerType schemaapi.CraftingSchema_Runner_RunnerType, jobURL string) *api.CraftingState {
	// Generate Crafting state
	return &api.CraftingState{
		InputSchema: schema,
		Attestation: &api.Attestation{
			InitializedAt: timestamppb.New(time.Now()),
			Workflow:      wf,
			RunnerType:    runnerType,
			RunnerUrl:     jobURL,
		},
		DryRun: dryRun,
	}
}

func persistCraftingState(craftState *api.CraftingState, stateFilePath string) error {
	marshaller := protojson.MarshalOptions{Indent: "  "}
	raw, err := marshaller.Marshal(craftState)
	if err != nil {
		return err
	}

	// Create empty file
	file, err := os.Create(stateFilePath)
	if err != nil {
		return err
	}

	_, err = file.Write(raw)
	if err != nil {
		return err
	}

	return nil
}

// ResolveEnvVars will iterate on the env vars in the allow list and resolve them from the system context
// strict indicates if it should fail if any env variable can not be found
func (c *Crafter) ResolveEnvVars(strict bool) error {
	if err := c.requireStateLoaded(); err != nil {
		return err
	}

	// Runner specific env variables
	c.logger.Debug().Str("runnerType", c.Runner.String()).Msg("loading runner specific env variables")
	var outputEnvVars = make(map[string]string)
	if !c.Runner.CheckEnv() {
		errorStr := fmt.Sprintf("couldn't detect the environment %q. Is the crafting process happening in the target env?", c.Runner.String())
		if strict {
			return fmt.Errorf("%s - %w", errorStr, ErrRunnerContextNotFound)
		}
		c.logger.Warn().Msg(errorStr)
	} else {
		c.logger.Debug().Str("runnerType", c.Runner.String()).Strs("variables", c.Runner.ListEnvVars()).Msg("list of env variables to automatically extract")
		outputEnvVars = c.Runner.ResolveEnvVars()
		if notFound := notResolvedVars(outputEnvVars, c.Runner.ListEnvVars()); len(notFound) > 0 {
			if strict {
				return fmt.Errorf("required env variables not present %q", notFound)
			}
			c.logger.Warn().Strs("key", notFound).Msg("required env variables not present")
		}
	}

	// User-defined env vars
	varsAllowList := c.CraftingState.InputSchema.EnvAllowList
	if len(varsAllowList) > 0 {
		c.logger.Debug().Strs("allowList", varsAllowList).Msg("loading env variables")
		for _, want := range varsAllowList {
			val := os.Getenv(want)
			if val == "" {
				continue
			}

			outputEnvVars[want] = val
		}

		if notFound := notResolvedVars(outputEnvVars, varsAllowList); len(notFound) > 0 {
			if strict {
				return fmt.Errorf("required env variables not present %q", notFound)
			}
			c.logger.Warn().Strs("key", notFound).Msg("required env variables not present")
		}
	}

	c.CraftingState.Attestation.EnvVars = outputEnvVars

	if err := persistCraftingState(c.CraftingState, c.statePath); err != nil {
		return err
	}

	return nil
}

func notResolvedVars(resolved map[string]string, wantList []string) []string {
	var notFound []string
	for _, want := range wantList {
		if val, ok := resolved[want]; !ok || val == "" {
			notFound = append(notFound, want)
		}
	}

	return notFound
}

// Inject material to attestation state
func (c *Crafter) AddMaterial(key, value string) error {
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
	mt, err := materials.Craft(context.Background(), m, value, c.uploader, c.logger)
	if err != nil {
		return err
	}

	if err := mt.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// 4 - Attach it to state
	if mt != nil {
		if c.CraftingState.Attestation.Materials == nil {
			c.CraftingState.Attestation.Materials = map[string]*api.Attestation_Material{key: mt}
		}
		c.CraftingState.Attestation.Materials[key] = mt
	}

	// 5 - Persist state
	if err := persistCraftingState(c.CraftingState, c.statePath); err != nil {
		return err
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
