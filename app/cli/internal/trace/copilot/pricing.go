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

package copilot

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
		panic(fmt.Sprintf("copilot trace: invalid embedded pricing: %v", err))
	}
	if _, ok := prices.Models[prices.Default]; !ok {
		panic("copilot trace: missing default model price")
	}
}

// normalizeModel maps a Copilot model name to a key of the price table:
// lowercase, no vendor prefix, and dashes in place of dots
// ("claude-sonnet-4.6" becomes "claude-sonnet-4-6").
func normalizeModel(model string) string {
	name := strings.ToLower(model)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.ReplaceAll(name, ".", "-")
}

// estimateCost returns the API-price estimate for the usage of one model.
// Input tokens exclude the cache tokens. An unknown model uses the default
// price and adds a warning.
func estimateCost(model string, u tokenUsage, warnings *[]string) float64 {
	price, ok := prices.Models[normalizeModel(model)]
	if !ok {
		price = prices.Models[prices.Default]
		*warnings = append(*warnings, fmt.Sprintf("unknown Copilot model %q; cost estimated using %s pricing", model, prices.Default))
	}
	writePrice := price.CacheWrite
	if writePrice == 0 {
		writePrice = price.Input
	}
	return (float64(u.Input)*price.Input + float64(u.CacheWrite)*writePrice + float64(u.CacheRead)*price.Cached + float64(u.Output)*price.Output) / 1e6
}

func roundCost(cost float64) float64 { return math.Round(cost*1e4) / 1e4 }
