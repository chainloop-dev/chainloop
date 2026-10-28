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

// Package orcarouter is the Chainloop client of OrcaRouter Cyber Inference, an
// OpenAI-compatible inference gateway with cyber models and gateway controls
// for security teams (https://www.orcarouter.ai/cyber).
//
// It holds the provider definition, the two credential adapters (a pasted API
// key and an OAuth 2.0 + PKCE login), the durable credential lifecycle, and
// the model catalog that both share. It is the single place where Chainloop
// talks to OrcaRouter, so no caller repeats the base URL, the environment
// variable, or the credential handling.
package orcarouter

// Identity of the provider. The name is what users see in listings and docs;
// the ID is the stable key used in configuration.
const (
	// Name is the display name of the provider.
	Name = "OrcaRouter Cyber Inference"
	// ID is the stable identifier of the provider.
	ID = "orcarouter"
	// BaseURL is the inference API. It is OpenAI-compatible and takes a
	// Bearer API key.
	BaseURL = "https://api.orcarouter.ai/v1"
	// AuthBaseURL is the origin of the consent screen and of the code
	// exchange. Authentication and inference use different public origins,
	// and the exchange path is /api/v1/auth/keys, not /v1/auth/keys.
	AuthBaseURL = "https://www.orcarouter.ai"
	// EnvKey is the environment variable that holds a pasted API key, the
	// convention of the other model providers Chainloop configures.
	EnvKey = "ORCAROUTER_API_KEY"
	// KeyPrefix is the visible prefix of an OrcaRouter API key. It is a
	// format check only, never a validity check.
	KeyPrefix = "sk-orca-"
	// DocsURL is the Cyber Inference provider page.
	DocsURL = "https://www.orcarouter.ai/cyber"
	// ModelsURL is the cyber tier model catalog.
	ModelsURL = "https://www.orcarouter.ai/models?tier=cyber"
	// ConsoleURL is where a user manages and revokes keys.
	ConsoleURL = "https://www.orcarouter.ai/console/authorized-apps"
)

// Models recommended by the provider documentation, in the order they are
// presented. They are model IDs of the OrcaRouter namespace; the values are
// recorded only for documentation and cold-start seeding, never as a substitute
// for the live catalog.
const (
	// DefaultModel routes automatically and works with any key.
	DefaultModel = "orcarouter/auto"
	// FallbackModel is the general-purpose fallback.
	FallbackModel = "orcarouter/fusion"

	// CyberUltraModel (OrcaCyber Ultra) is the dedicated cyber model for
	// authorized security teams, for security code scanning and multi-step
	// reasoning over large code bases. It needs business verification. The ID
	// keeps the namespace the live catalog publishes.
	CyberUltraModel = "orca/orcacyber-ultra-1.0"
	// CyberZeroModel (OrcaCyber Zero) is the OrcaCyber harness-driven cyber
	// model. It needs business verification.
	CyberZeroModel = "orca/orcacyber-zero-1.5"
)

// Provider is the definition that the harness and documentation layers consume.
// It exists so those layers take the base URL and the key variable from here
// instead of hardcoding a second copy.
type Provider struct {
	// ID is the stable identifier.
	ID string
	// Name is the display name.
	Name string
	// BaseURL is the OpenAI-compatible inference base URL.
	BaseURL string
	// EnvKey is the environment variable holding an API key, if any.
	EnvKey string
	// DefaultModel is the model used when the caller names none.
	DefaultModel string
}

// Default returns the OrcaRouter Cyber Inference provider definition.
func Default() Provider {
	return Provider{
		ID:           ID,
		Name:         Name,
		BaseURL:      BaseURL,
		EnvKey:       EnvKey,
		DefaultModel: DefaultModel,
	}
}

// RecommendedModels returns the model IDs that the provider documentation
// presents to users, in presentation order. See ModelsURL for the live list.
func RecommendedModels() []string {
	return []string{DefaultModel, CyberUltraModel, CyberZeroModel, FallbackModel}
}
