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
	"time"
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
			if _, err := time.Parse(time.DateOnly, name[len(name)-10:]); err == nil {
				price, ok = prices.Models[name[:len(name)-11]]
			}
		}
	}
	if !ok {
		price = prices.Models[prices.Default]
		*warnings = append(*warnings, fmt.Sprintf("unknown Codex model %q; cost estimated using %s pricing", model, prices.Default))
	}
	// Usage is normalized: base input, cache reads and cache writes are
	// disjoint. Reasoning is already included in output. Models without a
	// separate cache-write rate charge writes at the ordinary input rate.
	writePrice := price.CacheWrite
	if writePrice == 0 {
		writePrice = price.Input
	}
	return (float64(u.Input)*price.Input + float64(u.CacheWrite)*writePrice + float64(u.Cached)*price.Cached + float64(u.Output)*price.Output) / 1e6
}

func roundCost(cost float64) float64 { return math.Round(cost*1e4) / 1e4 }
