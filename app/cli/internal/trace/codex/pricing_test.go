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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizedUsagePricing(t *testing.T) {
	usage := tokenUsage{Input: 100, Cached: 800, CacheWrite: 100, Output: 50, Total: 150}
	for _, tc := range []struct {
		name, model string
		cost        float64
	}{
		{"separate cache write price", "gpt-6.1-sol", 0.00103},
		{"cache writes at input price", "gpt-5-mini", 0.00017},
		{"provider prefix", "openai/gpt-6.1-sol", 0.00103},
		{"date pinned model", "gpt-6.1-sol-2026-10-21", 0.00103},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []string
			assert.InDelta(t, tc.cost, estimateCost(tc.model, usage, &warnings), 0.000000001)
			assert.Empty(t, warnings)
		})
	}
}
