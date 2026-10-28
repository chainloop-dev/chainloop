//
// Copyright 2026 The Chainloop Authors.
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

package orcarouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Catalog limits. The response of a catalog endpoint is untrusted input, so it
// is bounded in bytes, items, and time before anything is kept.
const (
	// CatalogTimeout bounds a catalog request.
	CatalogTimeout = 20 * time.Second
	// MaxCatalogBytes bounds the catalog response.
	MaxCatalogBytes = 4 << 20
	// MaxCatalogItems bounds the number of records kept.
	MaxCatalogItems = 2000
)

// Capability is what an entry point can consume. A dropdown is built for one
// capability, so an embedding model never appears in a chat selector.
type Capability string

const (
	// CapabilityChat is text conversation.
	CapabilityChat Capability = "chat"
	// CapabilityEmbedding is vector embedding.
	CapabilityEmbedding Capability = "embedding"
	// CapabilityImage is image generation.
	CapabilityImage Capability = "image"
	// CapabilityVideo is video generation.
	CapabilityVideo Capability = "video"
	// CapabilityRerank is reranking.
	CapabilityRerank Capability = "rerank"
)

// Endpoint types of the catalog. A record advertises the wire protocols it can
// speak; only the ones the client can actually use are accepted.
var (
	textEndpointTypes = []string{"openai", "anthropic", "gemini", "openai-response"}
	// nonTextEndpoints mark models that are not general text chat even when
	// they accept a chat-shaped request.
	nonTextEndpoints = []string{"image-generation", "openai-video", "jina-rerank"}
)

// Model is one catalog record reduced to what a selector needs.
type Model struct {
	// ID is the model ID, with its vendor namespace kept verbatim.
	ID string `json:"id"`
	// Name is a human-readable label, when the catalog gives one.
	Name string `json:"name,omitempty"`
	// OwnedBy is the vendor the catalog attributes the model to.
	OwnedBy string `json:"owned_by,omitempty"`
	// EndpointTypes are the wire protocols the model speaks.
	EndpointTypes []string `json:"supported_endpoint_types,omitempty"`
	// EndpointCapabilities are the higher-level capabilities, when given.
	EndpointCapabilities []string `json:"endpoint_capabilities,omitempty"`
	// InputModalities are the modalities the model accepts. It is empty when
	// the catalog does not say, which means "not declared", not "text only".
	InputModalities []string `json:"input_modalities,omitempty"`
	// ReasoningEfforts are the effort levels the catalog verifies, when it
	// lists them.
	ReasoningEfforts []string `json:"reasoning_efforts,omitempty"`
}

// DeclaresInput reports whether the catalog explicitly declares a modality.
func (m Model) DeclaresInput(modality string) bool {
	return slices.Contains(m.InputModalities, modality)
}

// IsTextChat reports whether the model can serve a text chat entry point.
func (m Model) IsTextChat() bool {
	if !slices.ContainsFunc(m.EndpointTypes, func(t string) bool { return slices.Contains(textEndpointTypes, t) }) {
		return false
	}
	if slices.ContainsFunc(m.EndpointTypes, func(t string) bool { return slices.Contains(nonTextEndpoints, t) }) {
		return false
	}
	// A model that declares output modalities and none of them is text is not
	// a chat model, even if it accepts the chat wire format.
	return true
}

// Catalog is a model list plus where it came from.
type Catalog struct {
	// Models are the models that passed the filters of the capability.
	Models []Model
	// Source is "live" or "seed".
	Source CatalogSource
	// Degraded says the live discovery failed and the seed is in use.
	Degraded bool
	// Err is the discovery failure, when Degraded is true.
	Err error
}

// CatalogSource says where a model list came from.
type CatalogSource string

const (
	// SourceLive is the service catalog.
	SourceLive CatalogSource = "live"
	// SourceSeed is the verified cold-start list, used when discovery fails.
	SourceSeed CatalogSource = "seed"
)

// rawModel is the catalog record as received. It is a typed struct, and the
// nested architecture block is separate because the catalog may omit it.
type rawModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	OwnedBy                string   `json:"owned_by"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
	EndpointCapabilities   []string `json:"endpoint_capabilities"`
	Architecture           *struct {
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
	Reasoning struct {
		Efforts []string `json:"efforts"`
	} `json:"reasoning"`
}

// rawCatalog is the envelope of the catalog response.
type rawCatalog struct {
	Data []rawModel `json:"data"`
}

// DiscoverCatalog fetches the live catalog with a credential. A failure to reach
// the service is not fatal: the verified seed is returned, marked degraded, so a
// fresh installation still has a usable list.
func DiscoverCatalog(ctx context.Context, client *http.Client, baseURL string, cred Credential) Catalog {
	live, err := fetchCatalog(ctx, client, baseURL, cred)
	if err != nil {
		return Catalog{Models: seedModels(), Source: SourceSeed, Degraded: true, Err: err}
	}
	return Catalog{Models: live, Source: SourceLive}
}

// fetchCatalog performs the request and parses the records. It keeps the
// catalog authoritative on success and never merges the seed into it.
func fetchCatalog(ctx context.Context, client *http.Client, baseURL string, cred Credential) ([]Model, error) {
	if cred.Key == "" {
		return nil, errors.New("no OrcaRouter credential")
	}
	baseURL = NormalizeBase(baseURL)
	if baseURL == "" {
		baseURL = BaseURL
	}

	ctx, cancel := context.WithTimeout(ctx, CatalogTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Key)
	req.Header.Set("Accept", "application/json")

	if client == nil {
		client = &http.Client{Timeout: CatalogTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch the model catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrNeedsReauth
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, errors.New("the model catalog was rate limited (status 429)")
	default:
		return nil, fmt.Errorf("fetch the model catalog: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxCatalogBytes))
	if err != nil {
		return nil, fmt.Errorf("read the model catalog: %w", err)
	}

	var decoded rawCatalog
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode the model catalog: %w", err)
	}

	models := make([]Model, 0, min(len(decoded.Data), MaxCatalogItems))
	for _, record := range decoded.Data {
		if len(models) == MaxCatalogItems {
			break
		}
		model, ok := record.model()
		if !ok {
			continue
		}
		models = append(models, model)
	}
	return models, nil
}

// model converts a raw record, rejecting one that cannot be used at all.
func (m rawModel) model() (Model, bool) {
	id := strings.TrimSpace(m.ID)
	if id == "" {
		return Model{}, false
	}
	out := Model{
		ID:                   id,
		Name:                 m.Name,
		OwnedBy:              m.OwnedBy,
		EndpointTypes:        slices.Clone(m.SupportedEndpointTypes),
		EndpointCapabilities: slices.Clone(m.EndpointCapabilities),
		ReasoningEfforts:     slices.Clone(m.Reasoning.Efforts),
	}
	if m.Architecture != nil {
		out.InputModalities = slices.Clone(m.Architecture.InputModalities)
	}
	return out, true
}

// ForCapability filters a catalog for one entry point. An entry point that takes
// an attachment also requires the catalog to declare that input modality, so a
// model whose capabilities are unknown is left out rather than guessed at.
func ForCapability(models []Model, capability Capability, requiredInputs []string) []Model {
	out := make([]Model, 0, len(models))
	for _, model := range models {
		if !supports(model, capability) {
			continue
		}
		if !declaresAll(model, requiredInputs) {
			continue
		}
		out = append(out, model)
	}
	return out
}

// supports reports whether a model serves a capability.
func supports(model Model, capability Capability) bool {
	switch capability {
	case CapabilityChat:
		return model.IsTextChat()
	case CapabilityEmbedding:
		return matchesEndpoint(model, "embeddings") || slices.Contains(model.EndpointCapabilities, "embeddings")
	case CapabilityImage:
		return matchesEndpoint(model, "image-generation") || slices.Contains(model.EndpointCapabilities, "image_generation")
	case CapabilityVideo:
		return matchesEndpoint(model, "openai-video")
	case CapabilityRerank:
		return matchesEndpoint(model, "jina-rerank")
	default:
		return false
	}
}

// matchesEndpoint reports whether the model advertises an endpoint type.
func matchesEndpoint(model Model, endpoint string) bool {
	return slices.Contains(model.EndpointTypes, endpoint)
}

// declaresAll requires every modality to be explicitly declared. Empty
// requirements always pass.
func declaresAll(model Model, modalities []string) bool {
	for _, modality := range modalities {
		if !model.DeclaresInput(modality) {
			return false
		}
	}
	return true
}

// ChatModels is the chat selector list: text chat only, plus the required
// input modalities of the entry point.
func ChatModels(models []Model, requiredInputs []string) []Model {
	return ForCapability(models, CapabilityChat, requiredInputs)
}

// Contains reports whether a model ID is in a list.
func Contains(models []Model, id string) bool {
	return slices.ContainsFunc(models, func(m Model) bool { return m.ID == id })
}

// Revalidate returns the model ID if it is still offered by a filtered list, and
// the empty string otherwise. A restored selection must pass this before it is
// shown again.
func Revalidate(models []Model, id string) string {
	if id == "" || !Contains(models, id) {
		return ""
	}
	return id
}

// seedModels is the verified cold-start list. It is small on purpose, carries
// the metadata the catalog is known to publish, and is only used when live
// discovery fails. When live discovery succeeds it is not merged in.
func seedModels() []Model {
	return []Model{
		{
			ID:               "openai/gpt-5.5",
			Name:             "OpenAI: GPT-5.5",
			OwnedBy:          "OpenAI",
			EndpointTypes:    []string{"openai", "openai-response", "anthropic"},
			InputModalities:  []string{"text", "image"},
			ReasoningEfforts: []string{"low", "medium", "high", "xhigh"},
		},
		{
			ID:              "anthropic/claude-opus-4.8",
			Name:            "Anthropic: Claude Opus 4.8",
			OwnedBy:         "Anthropic",
			EndpointTypes:   []string{"openai", "openai-response", "anthropic"},
			InputModalities: []string{"text", "image"},
		},
		{
			ID:              "google/gemini-3.5-flash",
			Name:            "Google: Gemini 3.5 Flash",
			OwnedBy:         "Google",
			EndpointTypes:   []string{"openai", "openai-response", "gemini"},
			InputModalities: []string{"text", "image"},
		},
		{
			ID:              "deepseek/deepseek-v4-pro",
			Name:            "DeepSeek: DeepSeek V4 Pro",
			OwnedBy:         "DeepSeek",
			EndpointTypes:   []string{"openai", "openai-response"},
			InputModalities: []string{"text"},
		},
		{
			ID:              "orcarouter/auto",
			Name:            "OrcaRouter: Auto",
			OwnedBy:         "OrcaRouter",
			EndpointTypes:   []string{"openai", "openai-response", "anthropic", "gemini"},
			InputModalities: []string{"text"},
		},
		{
			ID:              "orcarouter/fusion",
			Name:            "OrcaRouter: Fusion",
			OwnedBy:         "OrcaRouter",
			EndpointTypes:   []string{"openai", "openai-response", "anthropic", "gemini"},
			InputModalities: []string{"text"},
		},
	}
}
