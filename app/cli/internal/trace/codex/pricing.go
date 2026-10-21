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

package codex

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type modelPricing struct {
	Input      float64 `json:"input"`
	Cached     float64 `json:"cached"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
}

//go:embed pricing.json
var pricingJSON []byte
var prices struct {
	Default string                  `json:"default_model"`
	Models  map[string]modelPricing `json:"models"`
}

func init() {
	if err := json.Unmarshal(pricingJSON, &prices); err != nil {
		panic(fmt.Sprintf("codex trace: invalid embedded pricing: %v", err))
	}
	if _, ok := prices.Models[prices.Default]; !ok {
		panic("codex trace: missing default model price")
	}
}

func estimateCost(model string, u tokenUsage, warnings *[]string) float64 {
	name := strings.TrimPrefix(model, "openai/")
	price, ok := prices.Models[name]
	if !ok {
		// Date-pinned model IDs use the same prices as their family alias.
		if len(name) > 11 && name[len(name)-11] == '-' {
			price, ok = prices.Models[name[:len(name)-11]]
		}
	}
	if !ok {
		price = prices.Models[prices.Default]
		*warnings = append(*warnings, fmt.Sprintf("unknown Codex model %q; cost estimated using %s pricing", model, prices.Default))
	}
	// Codex input_tokens already includes cached tokens. Cache writes are part
	// of input, and reasoning tokens are already part of output_tokens.
	uncached := max(0, u.Input-u.Cached)
	written := 0
	if price.CacheWrite > 0 {
		written = min(u.CacheWrite, uncached)
	}
	return (float64(uncached-written)*price.Input + float64(written)*price.CacheWrite + float64(u.Cached)*price.Cached + float64(u.Output)*price.Output) / 1e6
}

func roundCost(cost float64) float64 { return math.Round(cost*1e4) / 1e4 }
