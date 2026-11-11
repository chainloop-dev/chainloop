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

package claude

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// modelPricing holds per-million-token rates in USD for a Claude model.
type modelPricing struct {
	// Input is the price per 1M base input tokens.
	Input float64 `json:"input"`
	// Output is the price per 1M output tokens.
	Output float64 `json:"output"`
	// CacheWrite is the price per 1M tokens written to the 5-minute prompt cache tier (1.25x base input).
	CacheWrite float64 `json:"cache_write"`
	// CacheWrite1h is the price per 1M tokens written to the 1-hour prompt cache tier (2x base input).
	CacheWrite1h float64 `json:"cache_write_1h"`
	// CacheRead is the price per 1M tokens read from the prompt cache (0.1x base input on most models; see pricing.json notes for exceptions).
	CacheRead float64 `json:"cache_read"`
	// LongContextThreshold is the prompt size in tokens (input, cache reads and cache writes)
	// above which a request is priced at LongContext. Zero when the model has one price.
	LongContextThreshold int `json:"long_context_threshold,omitempty"`
	// LongContext holds the rates for requests whose prompt is larger than LongContextThreshold.
	LongContext *modelPricing `json:"long_context,omitempty"`
}

// cost returns the price in USD of the given usage at these rates.
func (p modelPricing) cost(u *tokenUsage) float64 {
	return (float64(u.InputTokens)*p.Input +
		float64(u.OutputTokens)*p.Output +
		float64(u.CacheCreation5mTokens)*p.CacheWrite +
		float64(u.CacheCreation1hTokens)*p.CacheWrite1h +
		float64(u.CacheReadInputTokens)*p.CacheRead) / 1_000_000
}

// pricingFile is the schema of the embedded pricing.json data file.
type pricingFile struct {
	Version      string                  `json:"version"`
	DefaultModel string                  `json:"default_model"`
	Models       map[string]modelPricing `json:"models"`
}

//go:embed pricing.json
var pricingJSON []byte

var (
	pricing        map[string]modelPricing
	defaultPricing modelPricing
	pricingVersion string
)

func init() {
	var f pricingFile
	if err := json.Unmarshal(pricingJSON, &f); err != nil {
		panic(fmt.Sprintf("claude trace: invalid embedded pricing.json: %v", err))
	}

	if len(f.Models) == 0 {
		panic("claude trace: embedded pricing.json has no models")
	}

	for model, p := range f.Models {
		if (p.LongContext != nil) != (p.LongContextThreshold > 0) {
			panic(fmt.Sprintf("claude trace: pricing.json model %q must set both long_context and long_context_threshold, or neither", model))
		}
	}

	def, ok := f.Models[f.DefaultModel]
	if !ok {
		panic(fmt.Sprintf("claude trace: default_model %q missing from pricing.json models", f.DefaultModel))
	}

	pricing = f.Models
	defaultPricing = def
	pricingVersion = f.Version
}
